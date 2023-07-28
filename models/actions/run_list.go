// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

type RunList []*ActionRun

// GetUserIDs returns a slice of user's id
func (runs RunList) GetUserIDs() []int64 {
	ids := make(container.Set[int64], len(runs))
	for _, run := range runs {
		ids.Add(run.TriggerUserID)
	}
	return ids.Values()
}

func (runs RunList) GetRepoIDs() []int64 {
	ids := make(container.Set[int64], len(runs))
	for _, run := range runs {
		ids.Add(run.RepoID)
	}
	return ids.Values()
}

func (runs RunList) LoadTriggerUser(ctx context.Context) error {
	userIDs := runs.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, run := range runs {
		if run.TriggerUserID == user_model.ActionsUserID {
			run.TriggerUser = user_model.NewActionsUser()
		} else {
			run.TriggerUser = users[run.TriggerUserID]
			if run.TriggerUser == nil {
				run.TriggerUser = user_model.NewGhostUser()
			}
		}
	}
	return nil
}

func (runs RunList) LoadRepos() error {
	repoIDs := runs.GetRepoIDs()
	repos, err := repo_model.GetRepositoriesMapByIDs(repoIDs)
	if err != nil {
		return err
	}
	for _, run := range runs {
		run.Repo = repos[run.RepoID]
	}
	return nil
}

type FindRunOptions struct {
	db.ListOptions
	RepoID           int64
	OwnerID          int64
	WorkflowFileName string
	TriggerUserID    int64
	Approved         bool // not util.OptionalBool, it works only when it's true
}

func (opts FindRunOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.WorkflowFileName != "" {
		cond = cond.And(builder.Eq{"workflow_id": opts.WorkflowFileName})
	}
	if opts.TriggerUserID > 0 {
		cond = cond.And(builder.Eq{"trigger_user_id": opts.TriggerUserID})
	}
	if opts.Approved {
		cond = cond.And(builder.Gt{"approved_by": 0})
	}
	return cond
}

func FindRuns(ctx context.Context, opts FindRunOptions) (RunList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize > 0 && opts.Page >= 1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var runs RunList
	total, err := e.Desc("id").FindAndCount(&runs)
	return runs, total, err
}

func CountRuns(ctx context.Context, opts FindRunOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(ActionRun))
}
