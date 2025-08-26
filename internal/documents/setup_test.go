package documents

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	cfg := &config.Settings{
		IsTest: true,
	}

	renderer := &render.HTMLRenderer{}
	handler := Setup(db, cfg, nil, renderer)

	assert.NotNil(t, handler)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupService(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	service := SetupService(db, nil)

	assert.NotNil(t, service)
	assert.NotNil(t, service.repo)
	assert.NoError(t, mock.ExpectationsWereMet())
}
