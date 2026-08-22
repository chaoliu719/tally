package bootstrap

import (
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// BuildEzbookkeepingConfig builds the minimal ezbookkeeping settings.Config
// needed to drive its services and datastore directly as a library, without
// going through ezbookkeeping's ini configuration file format.
func BuildEzbookkeepingConfig(cfg *Config) *settings.Config {
	return &settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType: settings.Sqlite3DbType,
			DatabasePath: cfg.DBPath,
		},
		UuidGeneratorType: settings.InternalUuidGeneratorType,
	}
}

// InitDataStore initializes the SQLite datastore (creating the file if it
// doesn't exist) and runs schema migrations for every model used by tally.
func InitDataStore(ezCfg *settings.Config) error {
	settings.SetCurrentConfig(ezCfg)

	if err := datastore.InitializeDataStore(ezCfg); err != nil {
		return err
	}

	if err := uuid.InitializeUuidGenerator(ezCfg); err != nil {
		return err
	}

	return syncTableStructures()
}

// syncTableStructures mirrors ezbookkeeping's own `database update` CLI
// command (cmd/database.go), syncing the complete set of tables ezbookkeeping
// defines so the resulting SQLite file stays schema-compatible with upstream
// tooling, even though tally only uses a subset of them today.
func syncTableStructures() error {
	userStoreModels := []any{
		new(models.User),
		new(models.TwoFactor),
		new(models.TwoFactorRecoveryCode),
	}

	for _, m := range userStoreModels {
		if err := datastore.Container.UserStore.SyncStructs(m); err != nil {
			return err
		}
	}

	if err := datastore.Container.TokenStore.SyncStructs(new(models.TokenRecord)); err != nil {
		return err
	}

	userDataStoreModels := []any{
		new(models.Account),
		new(models.Transaction),
		new(models.TransactionCategory),
		new(models.TransactionTagGroup),
		new(models.TransactionTag),
		new(models.TransactionTagIndex),
		new(models.TransactionTemplate),
		new(models.TransactionPictureInfo),
		new(models.UserCustomIcon),
		new(models.UserCustomExchangeRate),
		new(models.UserApplicationCloudSetting),
		new(models.UserExternalAuth),
		new(models.InsightsExplorer),
	}

	for _, m := range userDataStoreModels {
		if err := datastore.Container.UserDataStore.SyncStructs(m); err != nil {
			return err
		}
	}

	return nil
}
