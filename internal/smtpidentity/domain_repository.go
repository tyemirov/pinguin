package smtpidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListSenderDomainsForScope returns sender domains visible to the authenticated owner scope.
func (repository *Repository) ListSenderDomainsForScope(ctx context.Context, scope AccessScope) ([]SenderDomain, error) {
	var records []SenderDomain
	if err := repository.db.WithContext(ctx).
		Where(&SenderDomain{TenantID: scope.TenantID}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "domain"}}).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("smtp identity sender domains list: %w", err)
	}
	return records, nil
}

// CreateSenderDomainForScope registers a sender domain for DNS verification.
func (repository *Repository) CreateSenderDomainForScope(ctx context.Context, scope AccessScope, domain string) (SenderDomain, error) {
	normalizedDomain, normalizeErr := NormalizeSenderDomain(domain)
	if normalizeErr != nil {
		return SenderDomain{}, normalizeErr
	}
	if normalizedDomain == "" {
		return SenderDomain{}, ErrInvalidSenderDomain
	}
	var existing SenderDomain
	findErr := repository.db.WithContext(ctx).
		Where(&SenderDomain{Domain: normalizedDomain}).
		First(&existing).Error
	if findErr == nil {
		if existing.TenantID != scope.TenantID {
			return SenderDomain{}, ErrSenderDomainExists
		}
		return repository.ensureSenderDomainToken(ctx, existing)
	}
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return SenderDomain{}, fmt.Errorf("smtp identity sender domain lookup: %w", findErr)
	}
	token, tokenErr := repository.randomToken(domainVerificationBytes)
	if tokenErr != nil {
		return SenderDomain{}, tokenErr
	}
	now := repository.clockFunc()
	record := SenderDomain{
		TenantID:          scope.TenantID,
		Domain:            normalizedDomain,
		Status:            SenderDomainStatusPending,
		VerificationToken: token,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if createErr := repository.db.WithContext(ctx).Create(&record).Error; createErr != nil {
		var claimed SenderDomain
		if lookupErr := repository.db.WithContext(ctx).Where(&SenderDomain{Domain: normalizedDomain}).First(&claimed).Error; lookupErr == nil {
			if claimed.TenantID != scope.TenantID {
				return SenderDomain{}, ErrSenderDomainExists
			}
			return repository.ensureSenderDomainToken(ctx, claimed)
		}
		return SenderDomain{}, fmt.Errorf("smtp identity sender domain create: %w", createErr)
	}
	return record, nil
}

// RequireSenderDomainForScope returns one sender-domain setup record visible to the owner scope.
func (repository *Repository) RequireSenderDomainForScope(ctx context.Context, scope AccessScope, domainID uint) (SenderDomain, error) {
	var record SenderDomain
	query := repository.db.WithContext(ctx).Where(&SenderDomain{ID: domainID, TenantID: scope.TenantID})
	err := query.First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SenderDomain{}, ErrSenderDomainNotFound
		}
		return SenderDomain{}, fmt.Errorf("smtp identity sender domain lookup: %w", err)
	}
	return repository.ensureSenderDomainToken(ctx, record)
}

// UpdateSenderDomainStatusForScope stores the latest DNS verification outcome.
func (repository *Repository) UpdateSenderDomainStatusForScope(ctx context.Context, scope AccessScope, domainID uint, status SenderDomainStatus, checkedAt time.Time) (SenderDomain, error) {
	record, fetchErr := repository.RequireSenderDomainForScope(ctx, scope, domainID)
	if fetchErr != nil {
		return SenderDomain{}, fetchErr
	}
	record.Status = status
	record.LastCheckedAt = &checkedAt
	record.UpdatedAt = checkedAt
	if saveErr := repository.db.WithContext(ctx).Save(&record).Error; saveErr != nil {
		return SenderDomain{}, fmt.Errorf("smtp identity sender domain status: %w", saveErr)
	}
	return record, nil
}

func (repository *Repository) ensureSenderDomainToken(ctx context.Context, record SenderDomain) (SenderDomain, error) {
	if strings.TrimSpace(record.VerificationToken) != "" {
		return record, nil
	}
	token, tokenErr := repository.randomToken(domainVerificationBytes)
	if tokenErr != nil {
		return SenderDomain{}, tokenErr
	}
	record.VerificationToken = token
	record.UpdatedAt = repository.clockFunc()
	if saveErr := repository.db.WithContext(ctx).Save(&record).Error; saveErr != nil {
		return SenderDomain{}, fmt.Errorf("smtp identity sender domain token: %w", saveErr)
	}
	return record, nil
}
