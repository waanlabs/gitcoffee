// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
	cargo_service "code.gitea.io/gitea/services/packages/cargo"
	container_service "code.gitea.io/gitea/services/packages/container"
	debian_service "code.gitea.io/gitea/services/packages/debian"
)

// Cleanup removes expired package data
func Cleanup(taskCtx context.Context, olderThan time.Duration) error {
	ctx, committer, err := db.TxContext(taskCtx)
	if err != nil {
		return err
	}
	defer committer.Close()

	err = packages_model.IterateEnabledCleanupRules(ctx, func(ctx context.Context, pcr *packages_model.PackageCleanupRule) error {
		select {
		case <-taskCtx.Done():
			return db.ErrCancelledf("While processing package cleanup rules")
		default:
		}

		if err := pcr.CompiledPattern(); err != nil {
			return fmt.Errorf("CleanupRule [%d]: CompilePattern failed: %w", pcr.ID, err)
		}

		olderThan := time.Now().AddDate(0, 0, -pcr.RemoveDays)

		packages, err := packages_model.GetPackagesByType(ctx, pcr.OwnerID, pcr.Type)
		if err != nil {
			return fmt.Errorf("CleanupRule [%d]: GetPackagesByType failed: %w", pcr.ID, err)
		}

		anyVersionDeleted := false
		for _, p := range packages {
			pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
				PackageID:  p.ID,
				IsInternal: util.OptionalBoolFalse,
				Sort:       packages_model.SortCreatedDesc,
				Paginator:  db.NewAbsoluteListOptions(pcr.KeepCount, 200),
			})
			if err != nil {
				return fmt.Errorf("CleanupRule [%d]: SearchVersions failed: %w", pcr.ID, err)
			}
			versionDeleted := false
			for _, pv := range pvs {
				if pcr.Type == packages_model.TypeContainer {
					if skip, err := container_service.ShouldBeSkipped(ctx, pcr, p, pv); err != nil {
						return fmt.Errorf("CleanupRule [%d]: container.ShouldBeSkipped failed: %w", pcr.ID, err)
					} else if skip {
						log.Debug("Rule[%d]: keep '%s/%s' (container)", pcr.ID, p.Name, pv.Version)
						continue
					}
				}

				toMatch := pv.LowerVersion
				if pcr.MatchFullName {
					toMatch = p.LowerName + "/" + pv.LowerVersion
				}

				if pcr.KeepPatternMatcher != nil && pcr.KeepPatternMatcher.MatchString(toMatch) {
					log.Debug("Rule[%d]: keep '%s/%s' (keep pattern)", pcr.ID, p.Name, pv.Version)
					continue
				}
				if pv.CreatedUnix.AsLocalTime().After(olderThan) {
					log.Debug("Rule[%d]: keep '%s/%s' (remove days)", pcr.ID, p.Name, pv.Version)
					continue
				}
				if pcr.RemovePatternMatcher != nil && !pcr.RemovePatternMatcher.MatchString(toMatch) {
					log.Debug("Rule[%d]: keep '%s/%s' (remove pattern)", pcr.ID, p.Name, pv.Version)
					continue
				}

				log.Debug("Rule[%d]: remove '%s/%s'", pcr.ID, p.Name, pv.Version)

				if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
					return fmt.Errorf("CleanupRule [%d]: DeletePackageVersionAndReferences failed: %w", pcr.ID, err)
				}

				versionDeleted = true
				anyVersionDeleted = true
			}

			if versionDeleted {
				if pcr.Type == packages_model.TypeCargo {
					owner, err := user_model.GetUserByID(ctx, pcr.OwnerID)
					if err != nil {
						return fmt.Errorf("GetUserByID failed: %w", err)
					}
					if err := cargo_service.AddOrUpdatePackageIndex(ctx, owner, owner, p.ID); err != nil {
						return fmt.Errorf("CleanupRule [%d]: cargo.AddOrUpdatePackageIndex failed: %w", pcr.ID, err)
					}
				}
			}
		}

		if anyVersionDeleted {
			if pcr.Type == packages_model.TypeDebian {
				if err := debian_service.BuildAllRepositoryFiles(ctx, pcr.OwnerID); err != nil {
					return fmt.Errorf("CleanupRule [%d]: debian.BuildAllRepositoryFiles failed: %w", pcr.ID, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := container_service.Cleanup(ctx, olderThan); err != nil {
		return err
	}

	ps, err := packages_model.FindUnreferencedPackages(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypePackage, p.ID); err != nil {
			return err
		}
		if err := packages_model.DeletePackageByID(ctx, p.ID); err != nil {
			return err
		}
	}

	pbs, err := packages_model.FindExpiredUnreferencedBlobs(ctx, olderThan)
	if err != nil {
		return err
	}

	for _, pb := range pbs {
		if err := packages_model.DeleteBlobByID(ctx, pb.ID); err != nil {
			return err
		}
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	contentStore := packages_module.NewContentStore()
	for _, pb := range pbs {
		if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
			log.Error("Error deleting package blob [%v]: %v", pb.ID, err)
		}
	}

	return nil
}
