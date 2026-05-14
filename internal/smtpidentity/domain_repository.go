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
	query := repository.db.WithContext(ctx)
	if !scope.Admin {
		query = query.Where(clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: normalizeOwnerEmail(scope.OwnerEmail)})
	}
	if err := query.
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
	ownerEmail := normalizeOwnerEmail(scope.OwnerEmail)
	var existing SenderDomain
	findErr := repository.db.WithContext(ctx).
		Where(clause.Eq{Column: clause.Column{Name: "domain"}, Value: normalizedDomain}).
		First(&existing).Error
	if findErr == nil {
		if existing.OwnerEmail == ownerEmail || scope.Admin {
			return repository.ensureSenderDomainToken(ctx, existing)
		}
		return SenderDomain{}, fmt.Errorf("%w: %s", ErrSenderDomainExists, normalizedDomain)
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
		OwnerEmail:        ownerEmail,
		Domain:            normalizedDomain,
		Status:            SenderDomainStatusPending,
		VerificationToken: token,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if createErr := repository.db.WithContext(ctx).Create(&record).Error; createErr != nil {
		return SenderDomain{}, fmt.Errorf("smtp identity sender domain create: %w", createErr)
	}
	return record, nil
}

// RequireSenderDomainForScope returns one sender-domain setup record visible to the owner scope.
func (repository *Repository) RequireSenderDomainForScope(ctx context.Context, scope AccessScope, domainID uint) (SenderDomain, error) {
	var record SenderDomain
	query := repository.db.WithContext(ctx).Where(clause.Eq{Column: clause.Column{Name: identityIDColumn}, Value: domainID})
	if !scope.Admin {
		query = query.Where(clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: normalizeOwnerEmail(scope.OwnerEmail)})
	}
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

func normalizeOwnerEmail(email string) string {
	address, err := NewAddress(email)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(email))
	}
	return address.String()
}
