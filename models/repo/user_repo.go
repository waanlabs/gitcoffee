// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	api "code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
)

// GetStarredRepos returns the repos starred by a particular user
func GetStarredRepos(ctx context.Context, userID int64, private bool, listOptions db.ListOptions) ([]*Repository, error) {
	sess := db.GetEngine(ctx).
		Where("star.uid=?", userID).
		Join("LEFT", "star", "`repository`.id=`star`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)

		repos := make([]*Repository, 0, listOptions.PageSize)
		return repos, sess.Find(&repos)
	}

	repos := make([]*Repository, 0, 10)
	return repos, sess.Find(&repos)
}

// GetWatchedRepos returns the repos watched by a particular user
func GetWatchedRepos(ctx context.Context, userID int64, private bool, listOptions db.ListOptions) ([]*Repository, int64, error) {
	sess := db.GetEngine(ctx).
		Where("watch.user_id=?", userID).
		And("`watch`.mode<>?", WatchModeDont).
		Join("LEFT", "watch", "`repository`.id=`watch`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)

		repos := make([]*Repository, 0, listOptions.PageSize)
		total, err := sess.FindAndCount(&repos)
		return repos, total, err
	}

	repos := make([]*Repository, 0, 10)
	total, err := sess.FindAndCount(&repos)
	return repos, total, err
}

// GetRepoAssignees returns all users that have write access and can be assigned to issues
// of the repository,
func GetRepoAssignees(ctx context.Context, repo *Repository) (_ []*user_model.User, err error) {
	if err = repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	e := db.GetEngine(ctx)
	userIDs := make([]int64, 0, 10)
	if err = e.Table("access").
		Where("repo_id = ? AND mode >= ?", repo.ID, perm.AccessModeWrite).
		Select("user_id").
		Find(&userIDs); err != nil {
		return nil, err
	}

	additionalUserIDs := make([]int64, 0, 10)
	if err = e.Table("team_user").
		Join("INNER", "team_repo", "`team_repo`.team_id = `team_user`.team_id").
		Join("INNER", "team_unit", "`team_unit`.team_id = `team_user`.team_id").
		Where("`team_repo`.repo_id = ? AND `team_unit`.access_mode >= ?", repo.ID, perm.AccessModeWrite).
		Distinct("`team_user`.uid").
		Select("`team_user`.uid").
		Find(&additionalUserIDs); err != nil {
		return nil, err
	}

	uniqueUserIDs := make(container.Set[int64])
	uniqueUserIDs.AddMultiple(userIDs...)
	uniqueUserIDs.AddMultiple(additionalUserIDs...)

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*user_model.User, 0, len(uniqueUserIDs)+1)
	if len(userIDs) > 0 {
		if err = e.In("id", uniqueUserIDs.Values()).OrderBy(user_model.GetOrderByName()).Find(&users); err != nil {
			return nil, err
		}
	}
	if !repo.Owner.IsOrganization() && !uniqueUserIDs.Contains(repo.OwnerID) {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// GetReviewers get all users can be requested to review:
// * for private repositories this returns all users that have read access or higher to the repository.
// * for public repositories this returns all users that have read access or higher to the repository,
// all repo watchers and all organization members.
// TODO: may be we should have a busy choice for users to block review request to them.
func GetReviewers(ctx context.Context, repo *Repository, doerID, posterID int64) ([]*user_model.User, error) {
	// Get the owner of the repository - this often already pre-cached and if so saves complexity for the following queries
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	cond := builder.And(builder.Neq{"`user`.id": posterID})

	if repo.IsPrivate || repo.Owner.Visibility == api.VisibleTypePrivate {
		// This a private repository:
		// Anyone who can read the repository is a requestable reviewer

		cond = cond.And(builder.In("`user`.id",
			builder.Select("user_id").From("access").Where(
				builder.Eq{"repo_id": repo.ID}.
					And(builder.Gte{"mode": perm.AccessModeRead}),
			),
		))

		if repo.Owner.Type == user_model.UserTypeIndividual && repo.Owner.ID != posterID {
			// as private *user* repos don't generate an entry in the `access` table,
			// the owner of a private repo needs to be explicitly added.
			cond = cond.Or(builder.Eq{"`user`.id": repo.Owner.ID})
		}

	} else {
		// This is a "public" repository:
		// Any user that has read access, is a watcher or organization member can be requested to review
		cond = cond.And(builder.And(builder.In("`user`.id",
			builder.Select("user_id").From("access").
				Where(builder.Eq{"repo_id": repo.ID}.
					And(builder.Gte{"mode": perm.AccessModeRead})),
		).Or(builder.In("`user`.id",
			builder.Select("user_id").From("watch").
				Where(builder.Eq{"repo_id": repo.ID}.
					And(builder.In("mode", WatchModeNormal, WatchModeAuto))),
		).Or(builder.In("`user`.id",
			builder.Select("uid").From("org_user").
				Where(builder.Eq{"org_id": repo.OwnerID}),
		)))))
	}

	users := make([]*user_model.User, 0, 8)
	return users, db.GetEngine(ctx).Where(cond).OrderBy(user_model.GetOrderByName()).Find(&users)
}

// GetIssuePostersWithSearch returns users with limit of 30 whose username started with prefix that have authored an issue/pull request for the given repository
// If isShowFullName is set to true, also include full name prefix search
func GetIssuePostersWithSearch(ctx context.Context, repo *Repository, isPull bool, search string, isShowFullName bool) ([]*user_model.User, error) {
	users := make([]*user_model.User, 0, 30)
	var prefixCond builder.Cond = builder.Like{"name", search + "%"}
	if isShowFullName {
		prefixCond = prefixCond.Or(builder.Like{"full_name", "%" + search + "%"})
	}

	cond := builder.In("`user`.id",
		builder.Select("poster_id").From("issue").Where(
			builder.Eq{"repo_id": repo.ID}.
				And(builder.Eq{"is_pull": isPull}),
		).GroupBy("poster_id")).And(prefixCond)

	return users, db.GetEngine(ctx).
		Where(cond).
		Cols("id", "name", "full_name", "avatar", "avatar_email", "use_custom_avatar").
		OrderBy("name").
		Limit(30).
		Find(&users)
}
