// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

// InitDBEngine In case of problems connecting to DB, retry connection. Eg, PGSQL in Docker Container on Synology
func InitDBEngine(ctx context.Context) (err error) {
	log.Info("Beginning ORM engine initialization.")
	for i := 0; i < setting.Database.DBConnectRetries; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted due to shutdown:\nin retry ORM engine initialization")
		default:
		}
		log.Info("ORM engine initialization attempt #%d/%d...", i+1, setting.Database.DBConnectRetries)
		if err = db.InitEngineWithMigration(ctx, migrateWithSetting); err == nil {
			break
		} else if i == setting.Database.DBConnectRetries-1 {
			return err
		}
		log.Error("ORM engine initialization attempt #%d/%d failed. Error: %v", i+1, setting.Database.DBConnectRetries, err)
		log.Info("Backing off for %d seconds", int64(setting.Database.DBConnectBackoff/time.Second))
		time.Sleep(setting.Database.DBConnectBackoff)
	}
	db.HasEngine = true
	return nil
}

func migrateWithSetting(x *xorm.Engine) error {
	if setting.Database.AutoMigration {
		return migrations.Migrate(x)
	}

	if current, err := migrations.GetCurrentDBVersion(x); err != nil {
		return err
	} else if current < 0 {
		// execute migrations when the database isn't initialized even if AutoMigration is false
		return migrations.Migrate(x)
	} else if expected := migrations.ExpectedVersion(); current != expected {
		log.Fatal(`"database.AUTO_MIGRATION" is disabled, but current database version %d is not equal to the expected version %d.`+
			`You can set "database.AUTO_MIGRATION" to true or migrate manually by running "gitea [--config /path/to/app.ini] migrate"`, current, expected)
	}
	return nil
}
