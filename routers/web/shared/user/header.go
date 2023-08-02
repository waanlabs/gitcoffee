// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
)

// prepareContextForCommonProfile store some common data into context data for user's profile related pages (including the nav menu)
// It is designed to be fast and safe to be called multiple times in one request
func prepareContextForCommonProfile(ctx *context.Context) {
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["EnableFeed"] = setting.Other.EnableFeed
	ctx.Data["FeedURL"] = ctx.ContextUser.HomeLink()
}

// PrepareContextForProfileBigAvatar set the context for big avatar view on the profile page
func PrepareContextForProfileBigAvatar(ctx *context.Context) {
	prepareContextForCommonProfile(ctx)

	ctx.Data["IsFollowing"] = ctx.Doer != nil && user_model.IsFollowing(ctx.Doer.ID, ctx.ContextUser.ID)
	ctx.Data["ShowUserEmail"] = setting.UI.ShowUserEmail && ctx.ContextUser.Email != "" && ctx.IsSigned && !ctx.ContextUser.KeepEmailPrivate

	// Show OpenID URIs
	openIDs, err := user_model.GetUserOpenIDs(ctx.ContextUser.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openIDs

	if len(ctx.ContextUser.Description) != 0 {
		content, err := markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     map[string]string{"mode": "document"},
			GitRepo:   ctx.Repo.GitRepo,
			Ctx:       ctx,
		}, ctx.ContextUser.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = content
	}

	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	orgs, err := organization.FindOrgs(organization.FindOrgOptions{
		UserID:         ctx.ContextUser.ID,
		IncludePrivate: showPrivate,
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	ctx.Data["HasOrgsVisible"] = organization.HasOrgsVisible(orgs, ctx.Doer)

	badges, _, err := user_model.GetUserBadges(ctx, ctx.ContextUser)
	if err != nil {
		ctx.ServerError("GetUserBadges", err)
		return
	}
	ctx.Data["Badges"] = badges

	// in case the numbers are already provided by other functions, no need to query again (which is slow)
	if _, ok := ctx.Data["NumFollowers"]; !ok {
		_, ctx.Data["NumFollowers"], _ = user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{PageSize: 1, Page: 1})
	}
	if _, ok := ctx.Data["NumFollowing"]; !ok {
		_, ctx.Data["NumFollowing"], _ = user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{PageSize: 1, Page: 1})
	}
}

func FindUserProfileReadme(ctx *context.Context) (profileGitRepo *git.Repository, profileReadmeBlob *git.Blob, profileClose func()) {
	profileDbRepo, err := repo_model.GetRepositoryByName(ctx.ContextUser.ID, ".profile")
	if err == nil && !profileDbRepo.IsEmpty {
		if profileGitRepo, err = git.OpenRepository(ctx, profileDbRepo.RepoPath()); err != nil {
			log.Error("FindUserProfileReadme failed to OpenRepository: %v", err)
		} else {
			if commit, err := profileGitRepo.GetBranchCommit(profileDbRepo.DefaultBranch); err != nil {
				log.Error("FindUserProfileReadme failed to GetBranchCommit: %v", err)
			} else {
				profileReadmeBlob, _ = commit.GetBlobByPath("README.md")
			}
		}
	}
	return profileGitRepo, profileReadmeBlob, func() {
		if profileGitRepo != nil {
			_ = profileGitRepo.Close()
		}
	}
}

func RenderUserHeader(ctx *context.Context) {
	prepareContextForCommonProfile(ctx)

	_, profileReadmeBlob, profileClose := FindUserProfileReadme(ctx)
	defer profileClose()
	ctx.Data["HasProfileReadme"] = profileReadmeBlob != nil
}
