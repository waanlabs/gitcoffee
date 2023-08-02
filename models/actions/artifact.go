// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This artifact server is inspired by https://github.com/nektos/act/blob/master/pkg/artifacts/server.go.
// It updates url setting and uses ObjectStore to handle artifacts persistence.

package actions

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

const (
	// ArtifactStatusUploadPending is the status of an artifact upload that is pending
	ArtifactStatusUploadPending = 1
	// ArtifactStatusUploadConfirmed is the status of an artifact upload that is confirmed
	ArtifactStatusUploadConfirmed = 2
	// ArtifactStatusUploadError is the status of an artifact upload that is errored
	ArtifactStatusUploadError = 3
)

func init() {
	db.RegisterModel(new(ActionArtifact))
}

// ActionArtifact is a file that is stored in the artifact storage.
type ActionArtifact struct {
	ID                 int64 `xorm:"pk autoincr"`
	RunID              int64 `xorm:"index unique(runid_name_path)"` // The run id of the artifact
	RunnerID           int64
	RepoID             int64 `xorm:"index"`
	OwnerID            int64
	CommitSHA          string
	StoragePath        string             // The path to the artifact in the storage
	FileSize           int64              // The size of the artifact in bytes
	FileCompressedSize int64              // The size of the artifact in bytes after gzip compression
	ContentEncoding    string             // The content encoding of the artifact
	ArtifactPath       string             `xorm:"index unique(runid_name_path)"` // The path to the artifact when runner uploads it
	ArtifactName       string             `xorm:"index unique(runid_name_path)"` // The name of the artifact when runner uploads it
	Status             int64              `xorm:"index"`                         // The status of the artifact, uploading, expired or need-delete
	CreatedUnix        timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix        timeutil.TimeStamp `xorm:"updated index"`
}

func CreateArtifact(ctx context.Context, t *ActionTask, artifactName, artifactPath string) (*ActionArtifact, error) {
	if err := t.LoadJob(ctx); err != nil {
		return nil, err
	}
	artifact, err := getArtifactByNameAndPath(ctx, t.Job.RunID, artifactName, artifactPath)
	if errors.Is(err, util.ErrNotExist) {
		artifact := &ActionArtifact{
			ArtifactName: artifactName,
			ArtifactPath: artifactPath,
			RunID:        t.Job.RunID,
			RunnerID:     t.RunnerID,
			RepoID:       t.RepoID,
			OwnerID:      t.OwnerID,
			CommitSHA:    t.CommitSHA,
			Status:       ArtifactStatusUploadPending,
		}
		if _, err := db.GetEngine(ctx).Insert(artifact); err != nil {
			return nil, err
		}
		return artifact, nil
	} else if err != nil {
		return nil, err
	}
	return artifact, nil
}

func getArtifactByNameAndPath(ctx context.Context, runID int64, name, fpath string) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).Where("run_id = ? AND artifact_name = ? AND artifact_path = ?", runID, name, fpath).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}
	return &art, nil
}

// GetArtifactByID returns an artifact by id
func GetArtifactByID(ctx context.Context, id int64) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).ID(id).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}

	return &art, nil
}

// UpdateArtifactByID updates an artifact by id
func UpdateArtifactByID(ctx context.Context, id int64, art *ActionArtifact) error {
	art.ID = id
	_, err := db.GetEngine(ctx).ID(id).AllCols().Update(art)
	return err
}

// ListArtifactsByRunID returns all artifacts of a run
func ListArtifactsByRunID(ctx context.Context, runID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("run_id=?", runID).Find(&arts)
}

// ListArtifactsByRunIDAndArtifactName returns an artifacts of a run by artifact name
func ListArtifactsByRunIDAndArtifactName(ctx context.Context, runID int64, artifactName string) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("run_id=? AND artifact_name=?", runID, artifactName).Find(&arts)
}

// ListUploadedArtifactsByRunID returns all uploaded artifacts of a run
func ListUploadedArtifactsByRunID(ctx context.Context, runID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("run_id=? AND status=?", runID, ArtifactStatusUploadConfirmed).Find(&arts)
}

// ActionArtifactMeta is the meta data of an artifact
type ActionArtifactMeta struct {
	ArtifactName string
	FileSize     int64
}

// ListUploadedArtifactsMeta returns all uploaded artifacts meta of a run
func ListUploadedArtifactsMeta(ctx context.Context, runID int64) ([]*ActionArtifactMeta, error) {
	arts := make([]*ActionArtifactMeta, 0, 10)
	return arts, db.GetEngine(ctx).Table("action_artifact").
		Where("run_id=? AND status=?", runID, ArtifactStatusUploadConfirmed).
		GroupBy("artifact_name").
		Select("artifact_name, sum(file_size) as file_size").
		Find(&arts)
}

// ListArtifactsByRepoID returns all artifacts of a repo
func ListArtifactsByRepoID(ctx context.Context, repoID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("repo_id=?", repoID).Find(&arts)
}

// ListArtifactsByRunIDAndName returns artifacts by name of a run
func ListArtifactsByRunIDAndName(ctx context.Context, runID int64, name string) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("run_id=? AND artifact_name=?", runID, name).Find(&arts)
}
