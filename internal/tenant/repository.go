package tenant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"gorm.io/gorm"
)

// RuntimeConfig aggregates tenant data required at runtime.
type RuntimeConfig struct {
	Tenant Tenant
	Email  EmailCredentials
	SMS    *SMSCredentials
}

// EmailCredentials exposes decrypted SMTP settings.
type EmailCredentials struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
}

// SMSCredentials exposes decrypted Twilio settings.
type SMSCredentials struct {
	AccountSID string
	AuthToken  string
	FromNumber string
}

// ErrInvalidTenantID indicates the provided tenant identifier cannot be processed.
var ErrInvalidTenantID = errors.New("tenant: invalid tenant id")

// Repository exposes tenant lookups.
type Repository struct {
	db                *gorm.DB
	keeper            *SecretKeeper
	cacheMutex        sync.RWMutex
	runtimeCache      map[string]RuntimeConfig
	domainTenantCache map[string]string
}

var repositoryRegistry = struct {
	sync.Mutex
	repos map[*Repository]struct{}
}{
	repos: make(map[*Repository]struct{}),
}

// NewRepository constructs a repository.
func NewRepository(db *gorm.DB, keeper *SecretKeeper) *Repository {
	repo := &Repository{
		db:                db,
		keeper:            keeper,
		runtimeCache:      make(map[string]RuntimeConfig),
		domainTenantCache: make(map[string]string),
	}
	repositoryRegistry.Lock()
	repositoryRegistry.repos[repo] = struct{}{}
	repositoryRegistry.Unlock()
	return repo
}

// ResolveByHost returns the tenant associated with the provided host.
func (repo *Repository) ResolveByHost(ctx context.Context, host string) (RuntimeConfig, error) {
	normalized := normalizeHost(host)
	if normalized == "" {
		return RuntimeConfig{}, fmt.Errorf("tenant resolve: empty host")
	}
	if cachedTenantID, ok := repo.cachedTenantID(normalized); ok {
		return repo.runtimeConfig(ctx, cachedTenantID)
	}
	var domain TenantDomain
	if err := repo.db.WithContext(ctx).Where(&TenantDomain{Host: normalized}).First(&domain).Error; err != nil {
		return RuntimeConfig{}, fmt.Errorf("tenant resolve: domain %s: %w", normalized, err)
	}
	runtimeCfg, err := repo.runtimeConfig(ctx, domain.TenantID)
	if err != nil {
		return RuntimeConfig{}, err
	}
	repo.cacheTenantID(normalized, domain.TenantID)
	return runtimeCfg, nil
}

// ResolveByID fetches tenant runtime config by id.
func (repo *Repository) ResolveByID(ctx context.Context, tenantID string) (RuntimeConfig, error) {
	normalized := strings.TrimSpace(tenantID)
	if normalized == "" {
		return RuntimeConfig{}, fmt.Errorf("%w: empty tenant id", ErrInvalidTenantID)
	}
	return repo.runtimeConfig(ctx, normalized)
}

// ListActiveTenants returns active tenant rows.
func (repo *Repository) ListActiveTenants(ctx context.Context) ([]Tenant, error) {
	var tenants []Tenant
	if err := repo.db.WithContext(ctx).
		Where(&Tenant{Status: TenantStatusActive}).
		Find(&tenants).Error; err != nil {
		return nil, fmt.Errorf("tenant list: %w", err)
	}
	return tenants, nil
}

func (repo *Repository) runtimeConfig(ctx context.Context, tenantID string) (RuntimeConfig, error) {
	if cachedCfg, ok := repo.cachedRuntimeConfig(tenantID); ok {
		return cachedCfg, nil
	}
	loadedCfg, err := repo.loadRuntimeConfig(ctx, tenantID)
	if err != nil {
		return RuntimeConfig{}, err
	}
	repo.cacheRuntimeConfig(tenantID, loadedCfg)
	return cloneRuntimeConfig(loadedCfg), nil
}

func (repo *Repository) loadRuntimeConfig(ctx context.Context, tenantID string) (RuntimeConfig, error) {
	var tenantModel Tenant
	if err := repo.db.WithContext(ctx).Where(&Tenant{ID: tenantID}).First(&tenantModel).Error; err != nil {
		return RuntimeConfig{}, fmt.Errorf("tenant runtime: tenant %s: %w", tenantID, err)
	}
	var emailProfile EmailProfile
	if err := repo.db.WithContext(ctx).
		Where(&EmailProfile{TenantID: tenantID, IsDefault: true}).
		First(&emailProfile).Error; err != nil {
		return RuntimeConfig{}, fmt.Errorf("tenant runtime: email profile: %w", err)
	}
	var smsPtr *SMSCredentials
	var smsProfile SMSProfile
	if err := repo.db.WithContext(ctx).
		Where(&SMSProfile{TenantID: tenantID, IsDefault: true}).
		First(&smsProfile).Error; err == nil {
		accountSID, err := repo.keeper.Decrypt(smsProfile.AccountSIDCipher)
		if err != nil {
			return RuntimeConfig{}, err
		}
		authToken, err := repo.keeper.Decrypt(smsProfile.AuthTokenCipher)
		if err != nil {
			return RuntimeConfig{}, err
		}
		smsPtr = &SMSCredentials{
			AccountSID: accountSID,
			AuthToken:  authToken,
			FromNumber: smsProfile.FromNumber,
		}
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return RuntimeConfig{}, fmt.Errorf("tenant runtime: sms profile: %w", err)
	}
	username, err := repo.keeper.Decrypt(emailProfile.UsernameCipher)
	if err != nil {
		return RuntimeConfig{}, err
	}
	password, err := repo.keeper.Decrypt(emailProfile.PasswordCipher)
	if err != nil {
		return RuntimeConfig{}, err
	}
	return RuntimeConfig{
		Tenant: tenantModel,
		Email: EmailCredentials{
			Host:        emailProfile.Host,
			Port:        emailProfile.Port,
			Username:    username,
			Password:    password,
			FromAddress: emailProfile.FromAddress,
		},
		SMS: smsPtr,
	}, nil
}

func (repo *Repository) cachedRuntimeConfig(tenantID string) (RuntimeConfig, bool) {
	repo.cacheMutex.RLock()
	cachedCfg, ok := repo.runtimeCache[tenantID]
	repo.cacheMutex.RUnlock()
	if !ok {
		return RuntimeConfig{}, false
	}
	return cloneRuntimeConfig(cachedCfg), true
}

func (repo *Repository) cacheRuntimeConfig(tenantID string, cfg RuntimeConfig) {
	if tenantID == "" {
		return
	}
	repo.cacheMutex.Lock()
	repo.runtimeCache[tenantID] = cfg
	repo.cacheMutex.Unlock()
}

func (repo *Repository) clearCaches() {
	repo.cacheMutex.Lock()
	repo.runtimeCache = make(map[string]RuntimeConfig)
	repo.domainTenantCache = make(map[string]string)
	repo.cacheMutex.Unlock()
}

func (repo *Repository) cachedTenantID(host string) (string, bool) {
	repo.cacheMutex.RLock()
	tenantID, ok := repo.domainTenantCache[host]
	repo.cacheMutex.RUnlock()
	return tenantID, ok
}

func (repo *Repository) cacheTenantID(host string, tenantID string) {
	if host == "" || tenantID == "" {
		return
	}
	repo.cacheMutex.Lock()
	repo.domainTenantCache[host] = tenantID
	repo.cacheMutex.Unlock()
}

func cloneRuntimeConfig(cfg RuntimeConfig) RuntimeConfig {
	clonedCfg := cfg
	if cfg.SMS != nil {
		smsCopy := *cfg.SMS
		clonedCfg.SMS = &smsCopy
	}
	return clonedCfg
}

func invalidateRegisteredRepositories() {
	repositoryRegistry.Lock()
	defer repositoryRegistry.Unlock()
	for repo := range repositoryRegistry.repos {
		repo.clearCaches()
	}
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		return parts[0]
	}
	return host
}
