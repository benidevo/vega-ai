package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/benidevo/vega/internal/cache"
	commonerrors "github.com/benidevo/vega/internal/common/errors"
	"github.com/benidevo/vega/internal/db/querybuilder"
	"github.com/benidevo/vega/internal/job/models"
)

// companyColumns defines the columns selected for company queries.
var companyColumns = []string{"id", "name", "created_at", "updated_at"}

// SQLiteCompanyRepository is a SQLite implementation of CompanyRepository
type SQLiteCompanyRepository struct {
	db    *sql.DB
	cache cache.Cache
}

// NewSQLiteCompanyRepository creates a new SQLiteCompanyRepository instance
func NewSQLiteCompanyRepository(db *sql.DB, cache cache.Cache) *SQLiteCompanyRepository {
	return &SQLiteCompanyRepository{db: db, cache: cache}
}

// GetOrCreate retrieves a company by name or creates it if it doesn't exist
func (r *SQLiteCompanyRepository) GetOrCreate(ctx context.Context, name string) (*models.Company, error) {
	normalizedName, err := validateCompanyName(name)
	if err != nil {
		return nil, err
	}

	query, args, err := querybuilder.Select(companyColumns...).
		From("companies").
		Where("LOWER(name) = LOWER(?)", normalizedName).
		ToSql()
	if err != nil {
		return nil, wrapError(models.ErrCompanyNotFound, err)
	}

	var company models.Company
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&company.ID, &company.Name, &company.CreatedAt, &company.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create new company
		now := time.Now().UTC()
		insertQuery, insertArgs, err := querybuilder.Insert("companies").
			Columns("name", "created_at", "updated_at").
			Values(normalizedName, now, now).
			ToSql()
		if err != nil {
			return nil, wrapError(models.ErrFailedToCreateCompany, err)
		}

		result, err := r.db.ExecContext(ctx, insertQuery, insertArgs...)
		if err != nil {
			return nil, &commonerrors.RepositoryError{
				SentinelError: models.ErrFailedToCreateCompany,
				InnerError:    err,
			}
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, &commonerrors.RepositoryError{
				SentinelError: models.ErrFailedToCreateCompany,
				InnerError:    err,
			}
		}

		company = models.Company{
			ID:        int(id),
			Name:      normalizedName,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return &company, nil
	} else if err != nil {
		return nil, &commonerrors.RepositoryError{
			SentinelError: models.ErrCompanyNotFound,
			InnerError:    err,
		}
	}

	return &company, nil
}

// wrapError is a helper function to create a repository error
func wrapError(sentinel, inner error) error {
	return &commonerrors.RepositoryError{
		SentinelError: sentinel,
		InnerError:    inner,
	}
}

// GetByID retrieves a company by its ID
func (r *SQLiteCompanyRepository) GetByID(ctx context.Context, id int) (*models.Company, error) {
	if id <= 0 {
		return nil, models.ErrInvalidCompanyID
	}

	query, args, err := querybuilder.Select(companyColumns...).
		From("companies").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, wrapError(models.ErrCompanyNotFound, err)
	}

	var company models.Company
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&company.ID, &company.Name, &company.CreatedAt, &company.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrCompanyNotFound
		}
		return nil, wrapError(models.ErrCompanyNotFound, err)
	}
	return &company, nil
}

// GetByName retrieves a company by its name
func (r *SQLiteCompanyRepository) GetByName(ctx context.Context, name string) (*models.Company, error) {
	normalizedName, err := validateCompanyName(name)
	if err != nil {
		return nil, err
	}

	query, args, err := querybuilder.Select(companyColumns...).
		From("companies").
		Where("LOWER(name) = LOWER(?)", normalizedName).
		ToSql()
	if err != nil {
		return nil, wrapError(models.ErrCompanyNotFound, err)
	}

	var company models.Company
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&company.ID, &company.Name, &company.CreatedAt, &company.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrCompanyNotFound
		}
		return nil, wrapError(models.ErrCompanyNotFound, err)
	}

	return &company, nil
}

// validateCompanyName checks if the company name is valid and normalizes it
func validateCompanyName(name string) (string, error) {
	if name == "" {
		return "", models.ErrCompanyNameRequired
	}

	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return "", models.ErrCompanyNameRequired
	}

	return normalizedName, nil
}

// GetAll retrieves all companies from the database
func (r *SQLiteCompanyRepository) GetAll(ctx context.Context) ([]*models.Company, error) {
	query, args, err := querybuilder.Select(companyColumns...).
		From("companies").
		OrderBy("name").
		ToSql()
	if err != nil {
		return nil, wrapError(models.ErrFailedToCreateCompany, err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapError(models.ErrFailedToCreateCompany, err)
	}
	defer rows.Close()

	var companies []*models.Company
	for rows.Next() {
		var company models.Company
		err := rows.Scan(&company.ID, &company.Name, &company.CreatedAt, &company.UpdatedAt)
		if err != nil {
			return nil, wrapError(models.ErrFailedToCreateCompany, err)
		}
		companies = append(companies, &company)
	}

	if err = rows.Err(); err != nil {
		return nil, wrapError(models.ErrFailedToCreateCompany, err)
	}

	return companies, nil
}

// Delete removes a company from the database by ID
func (r *SQLiteCompanyRepository) Delete(ctx context.Context, id int) error {
	if id <= 0 {
		return models.ErrInvalidCompanyID
	}

	query, args, err := querybuilder.Delete("companies").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return wrapError(models.ErrFailedToDeleteCompany, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return wrapError(models.ErrFailedToDeleteCompany, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return wrapError(models.ErrFailedToDeleteCompany, err)
	}

	if rowsAffected == 0 {
		return models.ErrCompanyNotFound
	}

	return nil
}

// Update updates a company in the database
func (r *SQLiteCompanyRepository) Update(ctx context.Context, company *models.Company) error {
	if company == nil {
		return models.ErrInvalidCompanyID
	}

	if company.ID <= 0 {
		return models.ErrInvalidCompanyID
	}

	normalizedName, err := validateCompanyName(company.Name)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	query, args, err := querybuilder.Update("companies").
		Set("name", normalizedName).
		Set("updated_at", now).
		Where(sq.Eq{"id": company.ID}).
		ToSql()
	if err != nil {
		return wrapError(models.ErrFailedToUpdateCompany, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return wrapError(models.ErrFailedToUpdateCompany, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return wrapError(models.ErrFailedToUpdateCompany, err)
	}

	if rowsAffected == 0 {
		return models.ErrCompanyNotFound
	}

	company.Name = normalizedName
	company.UpdatedAt = now

	return nil
}
