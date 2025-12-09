package repository

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/benidevo/vega/internal/auth/models"
	commonerrors "github.com/benidevo/vega/internal/common/errors"
	"github.com/benidevo/vega/internal/db/querybuilder"
)

// UserRepository defines the interface for user-related data operations.
type UserRepository interface {
	// CreateUser inserts a new user into the database with the specified username,
	// password, and role, and returns the created User object.
	// It returns an error if the user creation fails.
	CreateUser(ctx context.Context, username, password, role string) (*models.User, error)

	// FindByUsername retrieves a user by their username.
	// It returns ErrUserNotFound if no user is found.
	FindByUsername(ctx context.Context, username string) (*models.User, error)

	// FindByID retrieves a user by their ID.
	// It returns ErrUserNotFound if no user is found.
	FindByID(ctx context.Context, id int) (*models.User, error)

	// UpdateUser updates the details of an existing user.
	// It returns ErrUserNotFound if the user does not exist.
	UpdateUser(ctx context.Context, user *models.User) (*models.User, error)

	// DeleteUser deletes a user by their ID.
	// It returns an error if the deletion fails.
	DeleteUser(ctx context.Context, id int) error

	// FindAllUsers retrieves all users from the database.
	// It returns an empty slice if no users are found.
	FindAllUsers(ctx context.Context) ([]*models.User, error)
}

// userColumns defines the columns selected for user queries.
var userColumns = []string{"id", "username", "password", "role", "created_at", "updated_at", "last_login"}

// SQLiteUserRepository provides methods to interact with the user data
// stored in an SQLite database.
type SQLiteUserRepository struct {
	db *sql.DB
}

// NewSQLiteUserRepository creates a new instance of SQLiteUserRepository
// with the provided database connection.
func NewSQLiteUserRepository(db *sql.DB) *SQLiteUserRepository {
	return &SQLiteUserRepository{db: db}
}

// CreateUser inserts a new user into the database with the specified username,
// password, and role, and returns the created User object.
// If a user with the given username already exists, it returns the existing user
// along with ErrUserAlreadyExists.
func (r *SQLiteUserRepository) CreateUser(ctx context.Context, username, password, role string) (*models.User, error) {
	existingUser, err := r.FindByUsername(ctx, username)
	if err == nil {
		return existingUser, models.ErrUserAlreadyExists
	} else if err != models.ErrUserNotFound {
		if _, ok := err.(*commonerrors.RepositoryError); !ok {
			return nil, models.WrapError(models.ErrUserCreationFailed, err)
		}
		return nil, err
	}

	roleValue, err := models.RoleFromString(role)
	if err != nil {
		return nil, models.WrapError(models.ErrInvalidRole, err)
	}

	// Start transaction for user and profile creation
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, models.WrapError(models.ErrUserCreationFailed, err)
	}
	defer tx.Rollback() // No-op if commit succeeds

	// Create user using Squirrel
	query, args, err := querybuilder.Insert("users").
		Columns("username", "password", "role").
		Values(username, password, roleValue).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrUserCreationFailed, err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: users.username" {
			existingUser, findErr := r.FindByUsername(ctx, username)
			if findErr == nil {
				return existingUser, models.ErrUserAlreadyExists
			}
			return nil, models.ErrUserAlreadyExists
		}
		return nil, models.WrapError(models.ErrUserCreationFailed, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, models.WrapError(models.ErrUserCreationFailed, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, models.WrapError(models.ErrUserCreationFailed, err)
	}

	return r.FindByID(ctx, int(id))
}

// FindByID retrieves a user by their ID from the SQLite database.
func (r *SQLiteUserRepository) FindByID(ctx context.Context, id int) (*models.User, error) {
	query, args, err := querybuilder.Select(userColumns...).
		From("users").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrUserRetrievalFailed, err)
	}

	return r.scanUser(r.db.QueryRowContext(ctx, query, args...))
}

// FindByUsername retrieves a user from the database by their username.
func (r *SQLiteUserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	query, args, err := querybuilder.Select(userColumns...).
		From("users").
		Where(sq.Eq{"username": username}).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrUserRetrievalFailed, err)
	}

	return r.scanUser(r.db.QueryRowContext(ctx, query, args...))
}

// UpdateUser updates an existing user's details in the database and returns the updated user.
func (r *SQLiteUserRepository) UpdateUser(ctx context.Context, user *models.User) (*models.User, error) {
	user.UpdatedAt = time.Now().UTC()

	updateBuilder := querybuilder.Update("users").
		Set("username", user.Username).
		Set("password", user.Password).
		Set("role", user.Role).
		Set("updated_at", user.UpdatedAt).
		Where(sq.Eq{"id": user.ID})

	if !user.LastLogin.IsZero() {
		updateBuilder = updateBuilder.Set("last_login", user.LastLogin)
	}

	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrUserUpdateFailed, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, models.WrapError(models.ErrUserUpdateFailed, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, models.WrapError(models.ErrUserUpdateFailed, err)
	}

	if rowsAffected == 0 {
		return nil, models.ErrUserNotFound
	}

	return r.FindByID(ctx, user.ID)
}

// DeleteUser removes a user from the database by their ID.
func (r *SQLiteUserRepository) DeleteUser(ctx context.Context, id int) error {
	query, args, err := querybuilder.Delete("users").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrUserDeletionFailed, err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrUserDeletionFailed, err)
	}
	return nil
}

// FindAllUsers retrieves all users from the database.
func (r *SQLiteUserRepository) FindAllUsers(ctx context.Context) ([]*models.User, error) {
	query, args, err := querybuilder.Select(userColumns...).
		From("users").
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrUserListRetrievalFailed, err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, models.WrapError(models.ErrUserListRetrievalFailed, err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, models.WrapError(models.ErrUserListRetrievalFailed, err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, models.WrapError(models.ErrUserListRetrievalFailed, err)
	}
	return users, nil
}

// scanUser scans a single row into a User struct.
func (r *SQLiteUserRepository) scanUser(row *sql.Row) (*models.User, error) {
	var user models.User
	var lastLogin sql.NullTime

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLogin,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrUserNotFound
		}
		return nil, models.WrapError(models.ErrUserRetrievalFailed, err)
	}

	if lastLogin.Valid {
		user.LastLogin = lastLogin.Time
	}

	return &user, nil
}

// scanUserFromRows scans a row from sql.Rows into a User struct.
func (r *SQLiteUserRepository) scanUserFromRows(rows *sql.Rows) (*models.User, error) {
	var user models.User
	var lastLogin sql.NullTime

	err := rows.Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLogin,
	)
	if err != nil {
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = lastLogin.Time
	}

	return &user, nil
}
