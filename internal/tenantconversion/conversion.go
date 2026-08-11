// Package tenantconversion contains the bounded operator-owned conversion from
// configured tenants to the managed tenant schema. It is not used by server startup.
package tenantconversion

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyemirov/pinguin/internal/db"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const (
	dispositionRetain    = "retain"
	dispositionDelete    = "delete"
	productionOwnerEmail = "temirov@gmail.com"
)

// SourceConfig is the strict former tenant YAML shape accepted by the conversion.
type SourceConfig struct {
	Tenants []SourceTenant `yaml:"tenants"`
}

// SourceTenant is one configured tenant before conversion.
type SourceTenant struct {
	ID           string             `yaml:"id"`
	DisplayName  string             `yaml:"displayName"`
	SupportEmail string             `yaml:"supportEmail"`
	Enabled      *bool              `yaml:"enabled"`
	Domains      []string           `yaml:"domains"`
	Admins       []string           `yaml:"admins"`
	EmailProfile SourceEmailProfile `yaml:"emailProfile"`
	SMSProfile   *SourceSMSProfile  `yaml:"smsProfile"`
}

// SourceEmailProfile is the former configured external SMTP profile.
type SourceEmailProfile struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	FromAddress string `yaml:"fromAddress"`
}

// SourceSMSProfile is the former configured Twilio profile.
type SourceSMSProfile struct {
	AccountSID string `yaml:"accountSid"`
	AuthToken  string `yaml:"authToken"`
	FromNumber string `yaml:"fromNumber"`
}

// Mapping assigns each source tenant a credential and each SMTP resource an outcome.
type Mapping struct {
	Owner             OwnerMapping          `yaml:"owner"`
	Tenants           []TenantMapping       `yaml:"tenants"`
	SMTPSenderDomains []SenderDomainMapping `yaml:"smtpSenderDomains"`
	SMTPIdentities    []IdentityMapping     `yaml:"smtpIdentities"`
}

// OwnerMapping identifies the single TAuth account that receives all source tenants.
type OwnerMapping struct {
	Email  string `yaml:"email"`
	UserID string `yaml:"userId"`
}

// TenantMapping gives one source tenant its new API credential.
type TenantMapping struct {
	SourceTenantID      string `yaml:"sourceTenantId"`
	APICredentialID     string `yaml:"apiCredentialId"`
	APICredentialDigest string `yaml:"apiCredentialDigest"`
}

// SenderDomainMapping retains or deletes one SMTP sender domain.
type SenderDomainMapping struct {
	ID                   uint   `yaml:"id"`
	Disposition          string `yaml:"disposition"`
	TargetSourceTenantID string `yaml:"targetSourceTenantId"`
}

// IdentityMapping retains or deletes one SMTP identity and its forwarding routes.
type IdentityMapping struct {
	ID                   string `yaml:"id"`
	Disposition          string `yaml:"disposition"`
	TargetSourceTenantID string `yaml:"targetSourceTenantId"`
}

// Result reports the verified conversion counts without exposing secrets.
type Result struct {
	Tenants           int
	Notifications     int
	Attachments       int
	SMTPSenderDomains int
	SMTPIdentities    int
	ForwardingRoutes  int
}

// DecodeSource parses a strict former tenant YAML document.
func DecodeSource(contents []byte) (SourceConfig, error) {
	var source SourceConfig
	if decodeErr := decodeStrictYAML(contents, &source); decodeErr != nil {
		return SourceConfig{}, fmt.Errorf("managed tenant conversion: source yaml: %w", decodeErr)
	}
	return source, nil
}

// DecodeMapping parses a strict owner, credential, and SMTP assignment document.
func DecodeMapping(contents []byte) (Mapping, error) {
	var mapping Mapping
	if decodeErr := decodeStrictYAML(contents, &mapping); decodeErr != nil {
		return Mapping{}, fmt.Errorf("managed tenant conversion: mapping yaml: %w", decodeErr)
	}
	return mapping, nil
}

func decodeStrictYAML(contents []byte, target interface{}) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
	decoder.KnownFields(true)
	if decodeErr := decoder.Decode(target); decodeErr != nil {
		return decodeErr
	}
	return nil
}

// Convert performs the validated schema and data conversion in one transaction.
func Convert(ctx context.Context, database *gorm.DB, keeper *tenant.SecretKeeper, source SourceConfig, mapping Mapping) (Result, error) {
	return convertWithOperations(ctx, database, keeper, source, mapping, defaultConversionOperations())
}

type conversionOperations struct {
	load           func(*gorm.DB) (legacyState, error)
	validate       func(SourceConfig, Mapping, legacyState) (validatedInputs, error)
	indexes        func(*gorm.DB) (map[string][]string, error)
	rename         func(*gorm.DB) error
	dropIndexes    func(*gorm.DB, map[string][]string) error
	createSchema   func(*gorm.DB) error
	copyState      func(*gorm.DB, secretEncryptor, legacyState, validatedInputs) (convertedState, error)
	dropTables     func(*gorm.DB) error
	validateSchema func(*gorm.DB) error
	verify         func(*gorm.DB, *tenant.SecretKeeper, convertedState) error
}

func defaultConversionOperations() conversionOperations {
	return conversionOperations{
		load: loadLegacyState, validate: validateInputs, indexes: legacyIndexes, rename: renameLegacyTables,
		dropIndexes: dropLegacyIndexes, createSchema: createManagedSchema, copyState: copyManagedState,
		dropTables: dropLegacyTables, validateSchema: db.ValidateManagedSchema, verify: verifyConvertedState,
	}
}

func convertWithOperations(ctx context.Context, database *gorm.DB, keeper *tenant.SecretKeeper, source SourceConfig, mapping Mapping, operations conversionOperations) (Result, error) {
	if database == nil || keeper == nil {
		return Result{}, errors.New("managed tenant conversion: database and secret keeper are required")
	}
	var result Result
	conversionErr := database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		state, loadErr := operations.load(transaction)
		if loadErr != nil {
			return loadErr
		}
		validated, validationErr := operations.validate(source, mapping, state)
		if validationErr != nil {
			return validationErr
		}
		indexes, indexErr := operations.indexes(transaction)
		if indexErr != nil {
			return indexErr
		}
		if renameErr := operations.rename(transaction); renameErr != nil {
			return renameErr
		}
		if dropErr := operations.dropIndexes(transaction, indexes); dropErr != nil {
			return dropErr
		}
		if schemaErr := operations.createSchema(transaction); schemaErr != nil {
			return schemaErr
		}
		converted, copyErr := operations.copyState(transaction, keeper, state, validated)
		if copyErr != nil {
			return copyErr
		}
		if dropErr := operations.dropTables(transaction); dropErr != nil {
			return dropErr
		}
		if schemaErr := operations.validateSchema(transaction); schemaErr != nil {
			return fmt.Errorf("managed tenant conversion: validate managed schema: %w", schemaErr)
		}
		if verifyErr := operations.verify(transaction, keeper, converted); verifyErr != nil {
			return verifyErr
		}
		result = converted.result
		return nil
	})
	if conversionErr != nil {
		return Result{}, conversionErr
	}
	return result, nil
}

type legacyTenant struct {
	ID           string `gorm:"primaryKey"`
	DisplayName  string
	SupportEmail string
	Status       string `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (legacyTenant) TableName() string { return "tenants" }

type legacyTenantDomain struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	Host      string `gorm:"uniqueIndex"`
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (legacyTenantDomain) TableName() string { return "tenant_domains" }

type legacyTenantAdmin struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  string `gorm:"index:idx_tenant_admin_email,unique"`
	Email     string `gorm:"index:idx_tenant_admin_email,unique;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (legacyTenantAdmin) TableName() string { return "tenant_admins" }

type legacyEmailProfile struct {
	ID             string `gorm:"primaryKey"`
	TenantID       string `gorm:"index"`
	Host           string
	Port           int
	UsernameCipher []byte
	PasswordCipher []byte
	FromAddress    string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (legacyEmailProfile) TableName() string { return "email_profiles" }

type legacySMSProfile struct {
	ID               string `gorm:"primaryKey"`
	TenantID         string `gorm:"index"`
	AccountSIDCipher []byte
	AuthTokenCipher  []byte
	FromNumber       string
	IsDefault        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (legacySMSProfile) TableName() string { return "sms_profiles" }

type legacySenderDomain struct {
	ID                uint                            `gorm:"primaryKey"`
	OwnerEmail        string                          `gorm:"index"`
	Domain            string                          `gorm:"uniqueIndex"`
	Status            smtpidentity.SenderDomainStatus `gorm:"index"`
	VerificationToken string
	LastCheckedAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (legacySenderDomain) TableName() string { return "sender_domains" }

type legacyIdentity struct {
	ID             string `gorm:"primaryKey"`
	OwnerEmail     string `gorm:"index"`
	EmailAddress   string `gorm:"uniqueIndex"`
	Username       string `gorm:"uniqueIndex"`
	PasswordSalt   []byte
	PasswordDigest []byte
	PasswordCipher []byte
	Status         smtpidentity.IdentityStatus `gorm:"index"`
	LastUsedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (legacyIdentity) TableName() string { return "identities" }

type legacyState struct {
	tenants           []legacyTenant
	domains           []legacyTenantDomain
	admins            []legacyTenantAdmin
	emailProfiles     []legacyEmailProfile
	smsProfiles       []legacySMSProfile
	notifications     []model.Notification
	attachments       []model.NotificationAttachment
	senderDomains     []legacySenderDomain
	identities        []legacyIdentity
	forwardRecipients []smtpidentity.ForwardRecipient
}

func loadLegacyState(database *gorm.DB) (legacyState, error) {
	expectedModels := []interface{}{
		&legacyTenant{}, &legacyTenantDomain{}, &legacyTenantAdmin{}, &legacyEmailProfile{}, &legacySMSProfile{},
		&model.Notification{}, &model.NotificationAttachment{}, &legacySenderDomain{}, &legacyIdentity{}, &smtpidentity.ForwardRecipient{},
	}
	if schemaErr := validateLegacySchema(database, expectedModels); schemaErr != nil {
		return legacyState{}, schemaErr
	}
	state := legacyState{}
	queries := []struct {
		label  string
		target interface{}
	}{
		{"tenants", &state.tenants}, {"tenant domains", &state.domains}, {"tenant admins", &state.admins},
		{"email profiles", &state.emailProfiles}, {"sms profiles", &state.smsProfiles}, {"notifications", &state.notifications},
		{"notification attachments", &state.attachments}, {"smtp sender domains", &state.senderDomains},
		{"smtp identities", &state.identities}, {"smtp forwarding routes", &state.forwardRecipients},
	}
	for _, query := range queries {
		if queryErr := database.Find(query.target).Error; queryErr != nil {
			return legacyState{}, fmt.Errorf("managed tenant conversion: load %s: %w", query.label, queryErr)
		}
	}
	return state, nil
}

func validateLegacySchema(database *gorm.DB, models []interface{}) error {
	tables, tableErr := database.Migrator().GetTables()
	if tableErr != nil {
		return fmt.Errorf("managed tenant conversion: inspect source schema: %w", tableErr)
	}
	actualTables := make(map[string]struct{}, len(tables))
	for _, tableName := range tables {
		if tableName != "sqlite_sequence" {
			actualTables[tableName] = struct{}{}
		}
	}
	return validateLegacySchemaTables(database, database.Migrator(), actualTables, models)
}

func validateLegacySchemaTables(database *gorm.DB, migrator gorm.Migrator, actualTables map[string]struct{}, models []interface{}) error {
	if len(actualTables) != len(models) {
		return fmt.Errorf("managed tenant conversion: source schema has %d application tables, expected %d", len(actualTables), len(models))
	}
	for _, schemaModel := range models {
		statement := &gorm.Statement{DB: database}
		if parseErr := statement.Parse(schemaModel); parseErr != nil {
			return fmt.Errorf("managed tenant conversion: parse source schema: %w", parseErr)
		}
		if _, available := actualTables[statement.Schema.Table]; !available {
			return fmt.Errorf("managed tenant conversion: source schema is missing table %s", statement.Schema.Table)
		}
		columnTypes, columnErr := migrator.ColumnTypes(schemaModel)
		if columnErr != nil {
			return fmt.Errorf("managed tenant conversion: inspect source table %s: %w", statement.Schema.Table, columnErr)
		}
		columns := make(map[string]struct{}, len(columnTypes))
		for _, columnType := range columnTypes {
			columns[columnType.Name()] = struct{}{}
		}
		if len(columns) != len(statement.Schema.DBNames) {
			return fmt.Errorf("managed tenant conversion: source table %s has %d columns, expected %d", statement.Schema.Table, len(columns), len(statement.Schema.DBNames))
		}
		for _, columnName := range statement.Schema.DBNames {
			if _, available := columns[columnName]; !available {
				return fmt.Errorf("managed tenant conversion: source table %s is missing column %s", statement.Schema.Table, columnName)
			}
		}
	}
	return nil
}

type validatedInputs struct {
	sourceByID      map[string]SourceTenant
	tenantMapping   map[string]validatedTenantMapping
	domainMapping   map[uint]SenderDomainMapping
	identityMapping map[string]IdentityMapping
}

type validatedTenantMapping struct {
	owner        tenant.OwnerUserID
	credentialID tenant.CredentialID
	digest       tenant.CredentialDigest
	newTenantID  string
}

func validateInputs(source SourceConfig, mapping Mapping, state legacyState) (validatedInputs, error) {
	owner, ownerErr := validateProductionOwner(mapping.Owner)
	if ownerErr != nil {
		return validatedInputs{}, ownerErr
	}
	validated := validatedInputs{
		sourceByID: make(map[string]SourceTenant, len(source.Tenants)), tenantMapping: make(map[string]validatedTenantMapping, len(mapping.Tenants)),
		domainMapping: make(map[uint]SenderDomainMapping, len(mapping.SMTPSenderDomains)), identityMapping: make(map[string]IdentityMapping, len(mapping.SMTPIdentities)),
	}
	for _, sourceTenant := range source.Tenants {
		sourceTenant.ID = strings.TrimSpace(sourceTenant.ID)
		if sourceTenant.ID == "" {
			return validatedInputs{}, errors.New("managed tenant conversion: each source tenant requires id")
		}
		if _, duplicate := validated.sourceByID[sourceTenant.ID]; duplicate {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: duplicate source tenant %s", sourceTenant.ID)
		}
		validated.sourceByID[sourceTenant.ID] = sourceTenant
	}
	if len(validated.sourceByID) != len(state.tenants) {
		return validatedInputs{}, errors.New("managed tenant conversion: source yaml and database tenant counts differ")
	}
	for _, storedTenant := range state.tenants {
		if _, available := validated.sourceByID[storedTenant.ID]; !available {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: database tenant %s is absent from source yaml", storedTenant.ID)
		}
	}
	credentialIDs := make(map[string]struct{})
	for _, tenantMapping := range mapping.Tenants {
		tenantMapping.SourceTenantID = strings.TrimSpace(tenantMapping.SourceTenantID)
		if _, available := validated.sourceByID[tenantMapping.SourceTenantID]; !available {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: mapping references unknown tenant %s", tenantMapping.SourceTenantID)
		}
		if _, duplicate := validated.tenantMapping[tenantMapping.SourceTenantID]; duplicate {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: duplicate tenant mapping %s", tenantMapping.SourceTenantID)
		}
		entry := validatedTenantMapping{}
		credentialID, credentialErr := tenant.NewCredentialID(tenantMapping.APICredentialID)
		digest, digestErr := tenant.ParseCredentialDigest(tenantMapping.APICredentialDigest)
		if credentialErr != nil || digestErr != nil {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: source tenant %s has invalid credential data", tenantMapping.SourceTenantID)
		}
		if _, duplicate := credentialIDs[credentialID.String()]; duplicate {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: duplicate API credential %s", credentialID.String())
		}
		credentialIDs[credentialID.String()] = struct{}{}
		entry.owner = owner
		entry.credentialID = credentialID
		entry.digest = digest
		entry.newTenantID = uuid.NewString()
		if profileErr := validateSourceProfiles(validated.sourceByID[tenantMapping.SourceTenantID]); profileErr != nil {
			return validatedInputs{}, profileErr
		}
		validated.tenantMapping[tenantMapping.SourceTenantID] = entry
	}
	if len(validated.tenantMapping) != len(validated.sourceByID) {
		return validatedInputs{}, errors.New("managed tenant conversion: every source tenant requires one mapping")
	}
	for _, domainMapping := range mapping.SMTPSenderDomains {
		if _, duplicate := validated.domainMapping[domainMapping.ID]; duplicate || domainMapping.ID == 0 {
			return validatedInputs{}, errors.New("managed tenant conversion: SMTP sender-domain mappings require unique nonzero ids")
		}
		if mappingErr := validateResourceMapping(domainMapping.Disposition, domainMapping.TargetSourceTenantID, validated.tenantMapping); mappingErr != nil {
			return validatedInputs{}, mappingErr
		}
		validated.domainMapping[domainMapping.ID] = domainMapping
	}
	if len(validated.domainMapping) != len(state.senderDomains) {
		return validatedInputs{}, errors.New("managed tenant conversion: every SMTP sender domain requires one mapping")
	}
	for _, domain := range state.senderDomains {
		if _, available := validated.domainMapping[domain.ID]; !available {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: SMTP sender domain %d is not mapped", domain.ID)
		}
	}
	for _, identityMapping := range mapping.SMTPIdentities {
		identityMapping.ID = strings.TrimSpace(identityMapping.ID)
		if _, duplicate := validated.identityMapping[identityMapping.ID]; duplicate || identityMapping.ID == "" {
			return validatedInputs{}, errors.New("managed tenant conversion: SMTP identity mappings require unique ids")
		}
		if mappingErr := validateResourceMapping(identityMapping.Disposition, identityMapping.TargetSourceTenantID, validated.tenantMapping); mappingErr != nil {
			return validatedInputs{}, mappingErr
		}
		validated.identityMapping[identityMapping.ID] = identityMapping
	}
	if len(validated.identityMapping) != len(state.identities) {
		return validatedInputs{}, errors.New("managed tenant conversion: every SMTP identity requires one mapping")
	}
	for _, identity := range state.identities {
		if _, available := validated.identityMapping[identity.ID]; !available {
			return validatedInputs{}, fmt.Errorf("managed tenant conversion: SMTP identity %s is not mapped", identity.ID)
		}
	}
	return validated, nil
}

func validateProductionOwner(mapping OwnerMapping) (tenant.OwnerUserID, error) {
	owner, ownerErr := tenant.NewOwnerUserID(mapping.UserID)
	ownerEmail, emailErr := tenant.NewSupportEmail(mapping.Email)
	if ownerErr != nil || emailErr != nil || ownerEmail.String() != productionOwnerEmail {
		return "", fmt.Errorf("managed tenant conversion: owner must identify the TAuth account %s", productionOwnerEmail)
	}
	return owner, nil
}

func validateSourceProfiles(sourceTenant SourceTenant) error {
	if _, displayErr := tenant.NewDisplayName(sourceTenant.DisplayName); displayErr != nil {
		return fmt.Errorf("managed tenant conversion: tenant %s has invalid display name", sourceTenant.ID)
	}
	if _, supportErr := tenant.NewSupportEmail(sourceTenant.SupportEmail); supportErr != nil {
		return fmt.Errorf("managed tenant conversion: tenant %s has invalid support email", sourceTenant.ID)
	}
	if _, emailErr := tenant.NewEmailProfileInput(sourceTenant.EmailProfile.Host, sourceTenant.EmailProfile.Port, sourceTenant.EmailProfile.Username, sourceTenant.EmailProfile.Password, sourceTenant.EmailProfile.FromAddress); emailErr != nil {
		return fmt.Errorf("managed tenant conversion: tenant %s has invalid email profile", sourceTenant.ID)
	}
	if sourceTenant.SMSProfile != nil {
		if _, smsErr := tenant.NewSMSProfileInput(sourceTenant.SMSProfile.AccountSID, sourceTenant.SMSProfile.AuthToken, sourceTenant.SMSProfile.FromNumber); smsErr != nil {
			return fmt.Errorf("managed tenant conversion: tenant %s has invalid SMS profile", sourceTenant.ID)
		}
	}
	return nil
}

func validateDisposition(disposition string) error {
	if disposition != dispositionRetain && disposition != dispositionDelete {
		return fmt.Errorf("managed tenant conversion: disposition must be %s or %s", dispositionRetain, dispositionDelete)
	}
	return nil
}

func validateResourceMapping(disposition string, targetSourceTenantID string, tenants map[string]validatedTenantMapping) error {
	if dispositionErr := validateDisposition(disposition); dispositionErr != nil {
		return dispositionErr
	}
	if disposition == dispositionDelete {
		if strings.TrimSpace(targetSourceTenantID) != "" {
			return errors.New("managed tenant conversion: deleted SMTP resources cannot have a target tenant")
		}
		return nil
	}
	_, available := tenants[strings.TrimSpace(targetSourceTenantID)]
	if !available {
		return errors.New("managed tenant conversion: retained SMTP resources require a retained target tenant")
	}
	return nil
}

var legacyTableNames = []string{
	"tenants", "tenant_domains", "tenant_admins", "email_profiles", "sms_profiles", "notifications", "notification_attachments", "sender_domains", "identities", "forward_recipients",
}

func conversionTableName(tableName string) string { return "managed_conversion_" + tableName }

func legacyIndexes(database *gorm.DB) (map[string][]string, error) {
	indexes := make(map[string][]string, len(legacyTableNames))
	for _, tableName := range legacyTableNames {
		tableIndexes, indexErr := database.Migrator().GetIndexes(tableName)
		if indexErr != nil {
			return nil, fmt.Errorf("managed tenant conversion: inspect indexes for %s: %w", tableName, indexErr)
		}
		for _, index := range tableIndexes {
			if !strings.HasPrefix(index.Name(), "sqlite_autoindex_") {
				indexes[tableName] = append(indexes[tableName], index.Name())
			}
		}
	}
	return indexes, nil
}

func renameLegacyTables(database *gorm.DB) error {
	for _, tableName := range legacyTableNames {
		if renameErr := database.Migrator().RenameTable(tableName, conversionTableName(tableName)); renameErr != nil {
			return fmt.Errorf("managed tenant conversion: rename table %s: %w", tableName, renameErr)
		}
	}
	return nil
}

func dropLegacyIndexes(database *gorm.DB, indexes map[string][]string) error {
	for tableName, tableIndexes := range indexes {
		for _, indexName := range tableIndexes {
			if dropErr := database.Migrator().DropIndex(conversionTableName(tableName), indexName); dropErr != nil {
				return fmt.Errorf("managed tenant conversion: drop source index %s: %w", indexName, dropErr)
			}
		}
	}
	return nil
}

func createManagedSchema(database *gorm.DB) error {
	if migrateErr := database.AutoMigrate(
		&model.Notification{}, &model.NotificationAttachment{}, &tenant.Tenant{}, &tenant.EmailProfile{}, &tenant.SMSProfile{},
		&tenant.APICredential{}, &tenant.IdempotencyRecord{}, &smtpidentity.SenderDomain{}, &smtpidentity.Identity{}, &smtpidentity.ForwardRecipient{},
	); migrateErr != nil {
		return fmt.Errorf("managed tenant conversion: create managed schema: %w", migrateErr)
	}
	return nil
}

type convertedState struct {
	result           Result
	tenantIDs        map[string]string
	emailCredentials map[string]tenant.EmailCredentials
	smsCredentials   map[string]*tenant.SMSCredentials
}

type secretEncryptor interface {
	Encrypt(string) ([]byte, error)
}

func copyManagedState(database *gorm.DB, keeper secretEncryptor, state legacyState, validated validatedInputs) (convertedState, error) {
	converted := convertedState{tenantIDs: make(map[string]string), emailCredentials: make(map[string]tenant.EmailCredentials), smsCredentials: make(map[string]*tenant.SMSCredentials)}
	storedTenants := make(map[string]legacyTenant, len(state.tenants))
	for _, storedTenant := range state.tenants {
		storedTenants[storedTenant.ID] = storedTenant
	}
	sourceIDs := make([]string, 0, len(validated.sourceByID))
	for sourceID := range validated.sourceByID {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Strings(sourceIDs)
	for _, sourceID := range sourceIDs {
		entry := validated.tenantMapping[sourceID]
		sourceTenant := validated.sourceByID[sourceID]
		storedTenant := storedTenants[sourceID]
		displayName, _ := tenant.NewDisplayName(sourceTenant.DisplayName)
		supportEmail, _ := tenant.NewSupportEmail(sourceTenant.SupportEmail)
		emailInput, _ := tenant.NewEmailProfileInput(sourceTenant.EmailProfile.Host, sourceTenant.EmailProfile.Port, sourceTenant.EmailProfile.Username, sourceTenant.EmailProfile.Password, sourceTenant.EmailProfile.FromAddress)
		usernameCipher, usernameErr := keeper.Encrypt(emailInput.Username)
		if usernameErr != nil {
			return convertedState{}, usernameErr
		}
		passwordCipher, passwordErr := keeper.Encrypt(emailInput.Password)
		if passwordErr != nil {
			return convertedState{}, passwordErr
		}
		managedTenant := tenant.Tenant{ID: entry.newTenantID, OwnerUserID: entry.owner.String(), DisplayName: displayName.String(), SupportEmail: supportEmail.String(), Version: 1, CreatedAt: storedTenant.CreatedAt, UpdatedAt: storedTenant.UpdatedAt}
		emailProfile := tenant.EmailProfile{ID: uuid.NewString(), TenantID: entry.newTenantID, Host: emailInput.Host, Port: emailInput.Port, UsernameCipher: usernameCipher, PasswordCipher: passwordCipher, FromAddress: emailInput.FromAddress, Version: 1, CreatedAt: storedTenant.CreatedAt, UpdatedAt: storedTenant.UpdatedAt}
		credential := tenant.APICredential{ID: entry.credentialID.String(), TenantID: entry.newTenantID, SecretDigest: entry.digest.Bytes(), DisplayPrefix: entry.credentialID.DisplayPrefix(), Version: 1, CreatedAt: storedTenant.CreatedAt, UpdatedAt: storedTenant.UpdatedAt}
		if createErr := database.Create(&managedTenant).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: create tenant %s: %w", sourceID, createErr)
		}
		if createErr := database.Create(&emailProfile).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: create email profile %s: %w", sourceID, createErr)
		}
		if createErr := database.Create(&credential).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: create API credential %s: %w", sourceID, createErr)
		}
		converted.tenantIDs[sourceID] = entry.newTenantID
		converted.emailCredentials[entry.newTenantID] = tenant.EmailCredentials(emailInput)
		if sourceTenant.SMSProfile != nil {
			smsInput, _ := tenant.NewSMSProfileInput(sourceTenant.SMSProfile.AccountSID, sourceTenant.SMSProfile.AuthToken, sourceTenant.SMSProfile.FromNumber)
			accountCipher, accountErr := keeper.Encrypt(smsInput.AccountSID)
			if accountErr != nil {
				return convertedState{}, accountErr
			}
			tokenCipher, tokenErr := keeper.Encrypt(smsInput.AuthToken)
			if tokenErr != nil {
				return convertedState{}, tokenErr
			}
			smsProfile := tenant.SMSProfile{ID: uuid.NewString(), TenantID: entry.newTenantID, AccountSIDCipher: accountCipher, AuthTokenCipher: tokenCipher, FromNumber: smsInput.FromNumber, Version: 1, CreatedAt: storedTenant.CreatedAt, UpdatedAt: storedTenant.UpdatedAt}
			if createErr := database.Create(&smsProfile).Error; createErr != nil {
				return convertedState{}, fmt.Errorf("managed tenant conversion: create SMS profile %s: %w", sourceID, createErr)
			}
			converted.smsCredentials[entry.newTenantID] = &tenant.SMSCredentials{AccountSID: smsInput.AccountSID, AuthToken: smsInput.AuthToken, FromNumber: smsInput.FromNumber}
		}
		converted.result.Tenants++
	}
	for _, notification := range state.notifications {
		newTenantID, retained := converted.tenantIDs[notification.TenantID]
		if !retained {
			continue
		}
		notification.TenantID = newTenantID
		notification.Attachments = nil
		if createErr := database.Create(&notification).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: copy notification %s: %w", notification.NotificationID, createErr)
		}
		converted.result.Notifications++
	}
	for _, attachment := range state.attachments {
		newTenantID, retained := converted.tenantIDs[attachment.TenantID]
		if !retained {
			continue
		}
		attachment.TenantID = newTenantID
		if createErr := database.Create(&attachment).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: copy notification attachment: %w", createErr)
		}
		converted.result.Attachments++
	}
	for _, domain := range state.senderDomains {
		assignment := validated.domainMapping[domain.ID]
		if assignment.Disposition == dispositionDelete {
			continue
		}
		managedDomain := smtpidentity.SenderDomain{ID: domain.ID, TenantID: converted.tenantIDs[assignment.TargetSourceTenantID], Domain: domain.Domain, Status: domain.Status, VerificationToken: domain.VerificationToken, LastCheckedAt: domain.LastCheckedAt, CreatedAt: domain.CreatedAt, UpdatedAt: domain.UpdatedAt}
		if createErr := database.Create(&managedDomain).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: copy SMTP sender domain %d: %w", domain.ID, createErr)
		}
		converted.result.SMTPSenderDomains++
	}
	retainedIdentities := make(map[string]struct{})
	for _, identity := range state.identities {
		assignment := validated.identityMapping[identity.ID]
		if assignment.Disposition == dispositionDelete {
			continue
		}
		managedIdentity := smtpidentity.Identity{ID: identity.ID, TenantID: converted.tenantIDs[assignment.TargetSourceTenantID], EmailAddress: identity.EmailAddress, Username: identity.Username, PasswordSalt: identity.PasswordSalt, PasswordDigest: identity.PasswordDigest, PasswordCipher: identity.PasswordCipher, Status: identity.Status, LastUsedAt: identity.LastUsedAt, CreatedAt: identity.CreatedAt, UpdatedAt: identity.UpdatedAt}
		if createErr := database.Create(&managedIdentity).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: copy SMTP identity %s: %w", identity.ID, createErr)
		}
		retainedIdentities[identity.ID] = struct{}{}
		converted.result.SMTPIdentities++
	}
	for _, route := range state.forwardRecipients {
		if _, retained := retainedIdentities[route.IdentityID]; !retained {
			continue
		}
		if createErr := database.Create(&route).Error; createErr != nil {
			return convertedState{}, fmt.Errorf("managed tenant conversion: copy forwarding route %s: %w", route.ID, createErr)
		}
		converted.result.ForwardingRoutes++
	}
	return converted, nil
}

func dropLegacyTables(database *gorm.DB) error {
	for tableIndex := len(legacyTableNames) - 1; tableIndex >= 0; tableIndex-- {
		tableName := conversionTableName(legacyTableNames[tableIndex])
		if dropErr := database.Migrator().DropTable(tableName); dropErr != nil {
			return fmt.Errorf("managed tenant conversion: drop source table %s: %w", tableName, dropErr)
		}
	}
	return nil
}

func verifyConvertedState(database *gorm.DB, keeper *tenant.SecretKeeper, converted convertedState) error {
	repository := tenant.NewRepository(database, keeper)
	for _, newTenantID := range converted.tenantIDs {
		runtimeConfig, runtimeErr := repository.ResolveByID(context.Background(), newTenantID)
		if runtimeErr != nil {
			return fmt.Errorf("managed tenant conversion: verify tenant %s: %w", newTenantID, runtimeErr)
		}
		if runtimeConfig.Email != converted.emailCredentials[newTenantID] {
			return fmt.Errorf("managed tenant conversion: email profile verification failed for %s", newTenantID)
		}
		expectedSMS := converted.smsCredentials[newTenantID]
		if (runtimeConfig.SMS == nil) != (expectedSMS == nil) || runtimeConfig.SMS != nil && *runtimeConfig.SMS != *expectedSMS {
			return fmt.Errorf("managed tenant conversion: SMS profile verification failed for %s", newTenantID)
		}
	}
	return nil
}
