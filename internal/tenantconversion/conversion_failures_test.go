package tenantconversion

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
)

type extraLegacyTable struct {
	ID uint `gorm:"primaryKey"`
}

func (extraLegacyTable) TableName() string { return "extra_legacy_table" }

type legacyTenantWithExtraColumn struct {
	legacyTenant
	Unexpected string
}

func (legacyTenantWithExtraColumn) TableName() string { return "tenants" }

type failingLegacyMigrator struct {
	gorm.Migrator
	columnErr error
}

func (migrator failingLegacyMigrator) ColumnTypes(interface{}) ([]gorm.ColumnType, error) {
	if migrator.columnErr != nil {
		return nil, migrator.columnErr
	}
	return migrator.Migrator.ColumnTypes(&legacyTenant{})
}

type conversionFailingEncryptor struct {
	calls  int
	failAt int
}

func (encryptor *conversionFailingEncryptor) Encrypt(value string) ([]byte, error) {
	encryptor.calls++
	if encryptor.calls == encryptor.failAt {
		return nil, errors.New("forced encryption failure")
	}
	return []byte("encrypted:" + value), nil
}

func TestConvertOperationFailures(t *testing.T) {
	keeper := conversionKeeper(t)
	if _, err := Convert(context.Background(), nil, keeper, SourceConfig{}, Mapping{}); err == nil {
		t.Fatal("expected nil database rejection")
	}
	database := openConversionDatabase(t)
	if _, err := Convert(context.Background(), database, nil, SourceConfig{}, Mapping{}); err == nil {
		t.Fatal("expected nil keeper rejection")
	}

	operationError := errors.New("forced conversion stage failure")
	newOperations := func() conversionOperations {
		return conversionOperations{
			load: func(*gorm.DB) (legacyState, error) { return legacyState{}, nil },
			validate: func(SourceConfig, Mapping, legacyState) (validatedInputs, error) {
				return validatedInputs{}, nil
			},
			indexes:      func(*gorm.DB) (map[string][]string, error) { return map[string][]string{}, nil },
			rename:       func(*gorm.DB) error { return nil },
			dropIndexes:  func(*gorm.DB, map[string][]string) error { return nil },
			createSchema: func(*gorm.DB) error { return nil },
			copyState: func(*gorm.DB, secretEncryptor, legacyState, validatedInputs) (convertedState, error) {
				return convertedState{result: Result{Tenants: 1}}, nil
			},
			dropTables:     func(*gorm.DB) error { return nil },
			validateSchema: func(*gorm.DB) error { return nil },
			verify:         func(*gorm.DB, *tenant.SecretKeeper, convertedState) error { return nil },
		}
	}
	for _, stage := range []string{"load", "validate", "indexes", "rename", "drop indexes", "create schema", "copy", "drop tables", "validate schema", "verify"} {
		t.Run(stage, func(t *testing.T) {
			operations := newOperations()
			switch stage {
			case "load":
				operations.load = func(*gorm.DB) (legacyState, error) { return legacyState{}, operationError }
			case "validate":
				operations.validate = func(SourceConfig, Mapping, legacyState) (validatedInputs, error) {
					return validatedInputs{}, operationError
				}
			case "indexes":
				operations.indexes = func(*gorm.DB) (map[string][]string, error) { return nil, operationError }
			case "rename":
				operations.rename = func(*gorm.DB) error { return operationError }
			case "drop indexes":
				operations.dropIndexes = func(*gorm.DB, map[string][]string) error { return operationError }
			case "create schema":
				operations.createSchema = func(*gorm.DB) error { return operationError }
			case "copy":
				operations.copyState = func(*gorm.DB, secretEncryptor, legacyState, validatedInputs) (convertedState, error) {
					return convertedState{}, operationError
				}
			case "drop tables":
				operations.dropTables = func(*gorm.DB) error { return operationError }
			case "validate schema":
				operations.validateSchema = func(*gorm.DB) error { return operationError }
			case "verify":
				operations.verify = func(*gorm.DB, *tenant.SecretKeeper, convertedState) error { return operationError }
			}
			result, err := convertWithOperations(context.Background(), openConversionDatabase(t), keeper, SourceConfig{}, Mapping{}, operations)
			if !errors.Is(err, operationError) || result != (Result{}) {
				t.Fatalf("stage %s = %+v, %v", stage, result, err)
			}
		})
	}
	success, err := convertWithOperations(context.Background(), openConversionDatabase(t), keeper, SourceConfig{}, Mapping{}, newOperations())
	if err != nil || success.Tenants != 1 {
		t.Fatalf("successful staged conversion = %+v, %v", success, err)
	}
}

func TestLegacySchemaAndLoadFailures(t *testing.T) {
	expectedModels := legacyModels()
	if err := validateLegacySchema(newLegacyDatabase(t), expectedModels); err != nil {
		t.Fatalf("current legacy schema: %v", err)
	}

	t.Run("table inspection", func(t *testing.T) {
		database := newLegacyDatabase(t)
		closeConversionDatabase(t, database)
		if err := validateLegacySchema(database, expectedModels); err == nil || !strings.Contains(err.Error(), "inspect source schema") {
			t.Fatalf("table inspection error = %v", err)
		}
	})
	t.Run("table count", func(t *testing.T) {
		database := newLegacyDatabase(t)
		if err := database.Migrator().CreateTable(&extraLegacyTable{}); err != nil {
			t.Fatalf("create extra table: %v", err)
		}
		if err := validateLegacySchema(database, expectedModels); err == nil || !strings.Contains(err.Error(), "application tables") {
			t.Fatalf("table count error = %v", err)
		}
	})
	t.Run("missing table", func(t *testing.T) {
		database := newLegacyDatabase(t)
		if err := database.Migrator().DropTable(&legacyTenantDomain{}); err != nil {
			t.Fatalf("drop legacy table: %v", err)
		}
		if err := database.Migrator().CreateTable(&extraLegacyTable{}); err != nil {
			t.Fatalf("create replacement table: %v", err)
		}
		if err := validateLegacySchema(database, expectedModels); err == nil || !strings.Contains(err.Error(), "missing table tenant_domains") {
			t.Fatalf("missing table error = %v", err)
		}
	})
	t.Run("parse model", func(t *testing.T) {
		database := openConversionDatabase(t)
		if err := validateLegacySchemaTables(database, database.Migrator(), map[string]struct{}{"invalid": {}}, []interface{}{make(chan int)}); err == nil || !strings.Contains(err.Error(), "parse source schema") {
			t.Fatalf("parse model error = %v", err)
		}
	})
	t.Run("column inspection", func(t *testing.T) {
		database := newLegacyDatabase(t)
		err := validateLegacySchemaTables(database, failingLegacyMigrator{Migrator: database.Migrator(), columnErr: errors.New("blocked")}, map[string]struct{}{"tenants": {}}, []interface{}{&legacyTenant{}})
		if err == nil || !strings.Contains(err.Error(), "inspect source table tenants") {
			t.Fatalf("column inspection error = %v", err)
		}
	})
	t.Run("column count", func(t *testing.T) {
		database := newLegacyDatabase(t)
		if err := database.AutoMigrate(&legacyTenantWithExtraColumn{}); err != nil {
			t.Fatalf("add legacy column: %v", err)
		}
		if err := validateLegacySchema(database, expectedModels); err == nil || !strings.Contains(err.Error(), "columns, expected") {
			t.Fatalf("column count error = %v", err)
		}
	})
	t.Run("missing column", func(t *testing.T) {
		database := newLegacyDatabase(t)
		if err := database.Migrator().RenameColumn(&legacyTenant{}, "display_name", "unexpected"); err != nil {
			t.Fatalf("rename legacy column: %v", err)
		}
		if err := validateLegacySchema(database, expectedModels); err == nil || !strings.Contains(err.Error(), "missing column display_name") {
			t.Fatalf("missing column error = %v", err)
		}
	})
	t.Run("load schema", func(t *testing.T) {
		if _, err := loadLegacyState(openConversionDatabase(t)); err == nil {
			t.Fatal("expected source schema load failure")
		}
	})
	t.Run("load query", func(t *testing.T) {
		database := newLegacyDatabase(t)
		callbackName := "pinguin:force_conversion_load_error"
		if err := database.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table == "tenants" {
				tx.AddError(errors.New("forced load failure"))
			}
		}); err != nil {
			t.Fatalf("register load callback: %v", err)
		}
		if _, err := loadLegacyState(database); err == nil || !strings.Contains(err.Error(), "load tenants") {
			t.Fatalf("load query error = %v", err)
		}
	})
}

func TestConversionInputValidationFailures(t *testing.T) {
	source, mapping, state := validValidationFixture()
	assertInvalid := func(t *testing.T, changedSource SourceConfig, changedMapping Mapping, changedState legacyState) {
		t.Helper()
		if _, err := validateInputs(changedSource, changedMapping, changedState); err == nil {
			t.Fatal("expected validation failure")
		}
	}

	tests := []struct {
		name   string
		mutate func(*SourceConfig, *Mapping, *legacyState)
	}{
		{name: "blank source id", mutate: func(source *SourceConfig, _ *Mapping, _ *legacyState) { source.Tenants[0].ID = " " }},
		{name: "duplicate source", mutate: func(source *SourceConfig, _ *Mapping, state *legacyState) {
			source.Tenants = append(source.Tenants, source.Tenants[0])
			state.tenants = append(state.tenants, legacyTenant{ID: "other"})
		}},
		{name: "source count", mutate: func(source *SourceConfig, _ *Mapping, _ *legacyState) { source.Tenants = nil }},
		{name: "stored source missing", mutate: func(_ *SourceConfig, _ *Mapping, state *legacyState) { state.tenants[0].ID = "other" }},
		{name: "unknown tenant mapping", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.Tenants[0].SourceTenantID = "other" }},
		{name: "duplicate tenant mapping", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) {
			mapping.Tenants = append(mapping.Tenants, mapping.Tenants[0])
		}},
		{name: "wrong production owner email", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.Owner.Email = "other@example.com" }},
		{name: "blank production owner id", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.Owner.UserID = " " }},
		{name: "invalid retained credentials", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.Tenants[0].APICredentialID = " " }},
		{name: "invalid retained profile", mutate: func(source *SourceConfig, _ *Mapping, _ *legacyState) { source.Tenants[0].EmailProfile.Host = "" }},
		{name: "missing tenant mapping", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.Tenants = nil }},
		{name: "zero domain mapping", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.SMTPSenderDomains[0].ID = 0 }},
		{name: "duplicate domain mapping", mutate: func(_ *SourceConfig, mapping *Mapping, state *legacyState) {
			mapping.SMTPSenderDomains = append(mapping.SMTPSenderDomains, mapping.SMTPSenderDomains[0])
			state.senderDomains = append(state.senderDomains, legacySenderDomain{ID: 2})
		}},
		{name: "invalid domain mapping", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) {
			mapping.SMTPSenderDomains[0].TargetSourceTenantID = "other"
		}},
		{name: "domain mapping count", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.SMTPSenderDomains = nil }},
		{name: "domain not mapped", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.SMTPSenderDomains[0].ID = 2 }},
		{name: "blank identity mapping", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.SMTPIdentities[0].ID = " " }},
		{name: "duplicate identity mapping", mutate: func(_ *SourceConfig, mapping *Mapping, state *legacyState) {
			mapping.SMTPIdentities = append(mapping.SMTPIdentities, mapping.SMTPIdentities[0])
			state.identities = append(state.identities, legacyIdentity{ID: "identity-2"})
		}},
		{name: "invalid identity mapping", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) {
			mapping.SMTPIdentities[0].TargetSourceTenantID = "other"
		}},
		{name: "identity mapping count", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.SMTPIdentities = nil }},
		{name: "identity not mapped", mutate: func(_ *SourceConfig, mapping *Mapping, _ *legacyState) { mapping.SMTPIdentities[0].ID = "identity-2" }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			changedSource, changedMapping, changedState := validValidationFixture()
			testCase.mutate(&changedSource, &changedMapping, &changedState)
			assertInvalid(t, changedSource, changedMapping, changedState)
		})
	}

	secondSource := source.Tenants[0]
	secondSource.ID = "tenant-two"
	secondMapping := mapping.Tenants[0]
	secondMapping.SourceTenantID = secondSource.ID
	state.tenants = append(state.tenants, legacyTenant{ID: secondSource.ID})
	source.Tenants = append(source.Tenants, secondSource)
	mapping.Tenants = append(mapping.Tenants, secondMapping)
	assertInvalid(t, source, mapping, state)

	for _, invalidTenant := range []SourceTenant{
		{ID: "tenant", SupportEmail: "support@example.com", EmailProfile: source.Tenants[0].EmailProfile},
		{ID: "tenant", DisplayName: "Name", SupportEmail: "bad", EmailProfile: source.Tenants[0].EmailProfile},
		{ID: "tenant", DisplayName: "Name", SupportEmail: "support@example.com", EmailProfile: SourceEmailProfile{}},
		{ID: "tenant", DisplayName: "Name", SupportEmail: "support@example.com", EmailProfile: source.Tenants[0].EmailProfile, SMSProfile: &SourceSMSProfile{}},
	} {
		if err := validateSourceProfiles(invalidTenant); err == nil {
			t.Fatalf("expected invalid source profile: %+v", invalidTenant)
		}
	}

	retained := map[string]validatedTenantMapping{"tenant": {}}
	for _, input := range []struct {
		disposition string
		target      string
		tenants     map[string]validatedTenantMapping
	}{
		{disposition: "archive", tenants: retained},
		{disposition: dispositionDelete, target: "tenant", tenants: retained},
		{disposition: dispositionRetain, target: "missing", tenants: retained},
	} {
		if err := validateResourceMapping(input.disposition, input.target, input.tenants); err == nil {
			t.Fatalf("expected invalid resource mapping: %+v", input)
		}
	}
	if err := validateResourceMapping(dispositionDelete, "", retained); err != nil {
		t.Fatalf("valid deleted mapping: %v", err)
	}
}

func TestConversionHelperAndCopyFailures(t *testing.T) {
	t.Run("legacy indexes", func(t *testing.T) {
		database := newLegacyDatabase(t)
		closeConversionDatabase(t, database)
		if _, err := legacyIndexes(database); err == nil {
			t.Fatal("expected index inspection failure")
		}
	})
	t.Run("rename tables", func(t *testing.T) {
		if err := renameLegacyTables(openConversionDatabase(t)); err == nil {
			t.Fatal("expected table rename failure")
		}
	})
	t.Run("drop indexes", func(t *testing.T) {
		if err := dropLegacyIndexes(openConversionDatabase(t), map[string][]string{"tenants": {"idx_missing"}}); err == nil {
			t.Fatal("expected index drop failure")
		}
	})
	t.Run("create schema", func(t *testing.T) {
		database := openConversionDatabase(t)
		closeConversionDatabase(t, database)
		if err := createManagedSchema(database); err == nil {
			t.Fatal("expected managed schema creation failure")
		}
	})
	t.Run("drop tables", func(t *testing.T) {
		database := openConversionDatabase(t)
		closeConversionDatabase(t, database)
		if err := dropLegacyTables(database); err == nil {
			t.Fatal("expected legacy table drop failure")
		}
	})

	state, validated := copyFixture(t)
	for failAt := 1; failAt <= 4; failAt++ {
		t.Run(fmt.Sprintf("encryption %d", failAt), func(t *testing.T) {
			database := openConversionDatabase(t)
			if err := createManagedSchema(database); err != nil {
				t.Fatalf("create managed schema: %v", err)
			}
			if _, err := copyManagedState(database, &conversionFailingEncryptor{failAt: failAt}, state, validated); err == nil {
				t.Fatal("expected encryption failure")
			}
		})
	}
	for _, tableName := range []string{"tenants", "email_profiles", "api_credentials", "sms_profiles", "notifications", "notification_attachments", "sender_domains", "identities", "forward_recipients"} {
		t.Run("create "+tableName, func(t *testing.T) {
			database := openConversionDatabase(t)
			if err := createManagedSchema(database); err != nil {
				t.Fatalf("create managed schema: %v", err)
			}
			registerConversionCreateError(t, database, tableName)
			if _, err := copyManagedState(database, conversionKeeper(t), state, validated); err == nil {
				t.Fatalf("expected %s create failure", tableName)
			}
		})
	}

	database := openConversionDatabase(t)
	if err := createManagedSchema(database); err != nil {
		t.Fatalf("create managed schema: %v", err)
	}
	converted, err := copyManagedState(database, conversionKeeper(t), state, validated)
	if err != nil {
		t.Fatalf("copy managed state: %v", err)
	}
	if converted.result != (Result{Tenants: 1, Notifications: 1, Attachments: 1, SMTPSenderDomains: 1, SMTPIdentities: 1, ForwardingRoutes: 1}) {
		t.Fatalf("copy result = %+v", converted.result)
	}
	keeper := conversionKeeper(t)
	if err := verifyConvertedState(database, keeper, converted); err != nil {
		t.Fatalf("verify copied state: %v", err)
	}
	badEmail := converted
	badEmail.emailCredentials = cloneEmailCredentials(converted.emailCredentials)
	for tenantID := range badEmail.emailCredentials {
		badEmail.emailCredentials[tenantID] = tenant.EmailCredentials{Host: "wrong"}
	}
	if err := verifyConvertedState(database, keeper, badEmail); err == nil || !strings.Contains(err.Error(), "email profile verification") {
		t.Fatalf("email verification error = %v", err)
	}
	badSMS := converted
	badSMS.smsCredentials = cloneSMSCredentials(converted.smsCredentials)
	for tenantID := range badSMS.smsCredentials {
		badSMS.smsCredentials[tenantID] = nil
	}
	if err := verifyConvertedState(database, keeper, badSMS); err == nil || !strings.Contains(err.Error(), "SMS profile verification") {
		t.Fatalf("SMS verification error = %v", err)
	}
	if err := verifyConvertedState(openConversionDatabase(t), keeper, convertedState{tenantIDs: map[string]string{"source": "missing"}}); err == nil || !strings.Contains(err.Error(), "verify tenant") {
		t.Fatalf("runtime verification error = %v", err)
	}
}

func validValidationFixture() (SourceConfig, Mapping, legacyState) {
	digest := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	source := SourceConfig{Tenants: []SourceTenant{{
		ID: "tenant-one", DisplayName: "Tenant One", SupportEmail: "support@example.com",
		EmailProfile: SourceEmailProfile{Host: "smtp.example.com", Port: 587, Username: "user", Password: "password", FromAddress: "sender@example.com"},
		SMSProfile:   &SourceSMSProfile{AccountSID: "AC123", AuthToken: "token", FromNumber: "+15551234567"},
	}}}
	mapping := Mapping{
		Owner:             OwnerMapping{Email: productionOwnerEmail, UserID: "owner-user"},
		Tenants:           []TenantMapping{{SourceTenantID: "tenant-one", APICredentialID: "11111111-1111-4111-8111-111111111111", APICredentialDigest: digest}},
		SMTPSenderDomains: []SenderDomainMapping{{ID: 1, Disposition: dispositionRetain, TargetSourceTenantID: "tenant-one"}},
		SMTPIdentities:    []IdentityMapping{{ID: "identity-one", Disposition: dispositionRetain, TargetSourceTenantID: "tenant-one"}},
	}
	state := legacyState{
		tenants:       []legacyTenant{{ID: "tenant-one"}},
		senderDomains: []legacySenderDomain{{ID: 1}},
		identities:    []legacyIdentity{{ID: "identity-one"}},
	}
	return source, mapping, state
}

func copyFixture(t *testing.T) (legacyState, validatedInputs) {
	t.Helper()
	source, mapping, state := validValidationFixture()
	now := time.Now().UTC()
	state.tenants[0].CreatedAt = now
	state.tenants[0].UpdatedAt = now
	state.notifications = []model.Notification{
		{TenantID: "tenant-one", NotificationID: "notification-one", NotificationType: model.NotificationEmail, Recipient: "recipient@example.com", Message: "message", Status: model.StatusSent, CreatedAt: now, UpdatedAt: now},
		{TenantID: "deleted", NotificationID: "notification-delete", NotificationType: model.NotificationEmail, Recipient: "recipient@example.com", Message: "message", Status: model.StatusSent, CreatedAt: now, UpdatedAt: now},
	}
	state.attachments = []model.NotificationAttachment{
		{TenantID: "tenant-one", NotificationID: "notification-one", Filename: "file.txt", ContentType: "text/plain", Data: []byte("data"), CreatedAt: now, UpdatedAt: now},
		{TenantID: "deleted", NotificationID: "notification-delete", Filename: "deleted.txt", ContentType: "text/plain", Data: []byte("data"), CreatedAt: now, UpdatedAt: now},
	}
	state.senderDomains[0] = legacySenderDomain{ID: 1, Domain: "example.com", Status: smtpidentity.SenderDomainStatusVerified, VerificationToken: "token", CreatedAt: now, UpdatedAt: now}
	state.identities[0] = legacyIdentity{ID: "identity-one", EmailAddress: "sender@example.com", Username: "smtp-user", PasswordSalt: []byte("salt"), PasswordDigest: []byte("digest"), PasswordCipher: []byte("cipher"), Status: smtpidentity.IdentityStatusActive, CreatedAt: now, UpdatedAt: now}
	state.forwardRecipients = []smtpidentity.ForwardRecipient{
		{ID: "route-one", IdentityID: "identity-one", EmailAddress: "owner@example.com", CreatedAt: now, UpdatedAt: now},
		{ID: "route-deleted", IdentityID: "identity-delete", EmailAddress: "deleted@example.com", CreatedAt: now, UpdatedAt: now},
	}
	validated, err := validateInputs(source, mapping, state)
	if err != nil {
		t.Fatalf("validate copy fixture: %v", err)
	}
	return state, validated
}

func legacyModels() []interface{} {
	return []interface{}{
		&legacyTenant{}, &legacyTenantDomain{}, &legacyTenantAdmin{}, &legacyEmailProfile{}, &legacySMSProfile{},
		&model.Notification{}, &model.NotificationAttachment{}, &legacySenderDomain{}, &legacyIdentity{}, &smtpidentity.ForwardRecipient{},
	}
}

func openConversionDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "conversion.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open conversion database: %v", err)
	}
	return database
}

func closeConversionDatabase(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	if err := sqlDatabase.Close(); err != nil {
		t.Fatalf("close conversion database: %v", err)
	}
}

func registerConversionCreateError(t *testing.T, database *gorm.DB, tableName string) {
	t.Helper()
	callbackName := "pinguin:force_conversion_create_error_" + tableName
	if err := database.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == tableName {
			tx.AddError(errors.New("forced conversion create failure"))
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
}

func cloneEmailCredentials(source map[string]tenant.EmailCredentials) map[string]tenant.EmailCredentials {
	result := make(map[string]tenant.EmailCredentials, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneSMSCredentials(source map[string]*tenant.SMSCredentials) map[string]*tenant.SMSCredentials {
	result := make(map[string]*tenant.SMSCredentials, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
