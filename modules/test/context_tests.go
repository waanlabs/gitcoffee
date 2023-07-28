// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	gocontext "context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web/middleware"

	chi "github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// MockContext mock context for unit tests
// TODO: move this function to other packages, because it depends on "models" package
func MockContext(t *testing.T, path string) *context.Context {
	resp := httptest.NewRecorder()
	requestURL, err := url.Parse(path)
	assert.NoError(t, err)
	req := &http.Request{
		URL:  requestURL,
		Form: url.Values{},
	}

	base, baseCleanUp := context.NewBaseContext(resp, req)
	base.Data = middleware.ContextData{}
	base.Locale = &translation.MockLocale{}
	ctx := &context.Context{
		Base:   base,
		Render: &mockRender{},
		Flash:  &middleware.Flash{Values: url.Values{}},
	}
	_ = baseCleanUp // during test, it doesn't need to do clean up. TODO: this can be improved later

	chiCtx := chi.NewRouteContext()
	ctx.Base.AppendContextValue(chi.RouteCtxKey, chiCtx)
	return ctx
}

// MockAPIContext mock context for unit tests
// TODO: move this function to other packages, because it depends on "models" package
func MockAPIContext(t *testing.T, path string) *context.APIContext {
	resp := httptest.NewRecorder()
	requestURL, err := url.Parse(path)
	assert.NoError(t, err)
	req := &http.Request{
		URL:  requestURL,
		Form: url.Values{},
	}

	base, baseCleanUp := context.NewBaseContext(resp, req)
	base.Data = middleware.ContextData{}
	base.Locale = &translation.MockLocale{}
	ctx := &context.APIContext{Base: base}
	_ = baseCleanUp // during test, it doesn't need to do clean up. TODO: this can be improved later

	chiCtx := chi.NewRouteContext()
	ctx.Base.AppendContextValue(chi.RouteCtxKey, chiCtx)
	return ctx
}

// LoadRepo load a repo into a test context.
func LoadRepo(t *testing.T, ctx gocontext.Context, repoID int64) {
	var doer *user_model.User
	repo := &context.Repository{}
	switch ctx := ctx.(type) {
	case *context.Context:
		ctx.Repo = repo
		doer = ctx.Doer
	case *context.APIContext:
		ctx.Repo = repo
		doer = ctx.Doer
	default:
		assert.Fail(t, "context is not *context.Context or *context.APIContext")
		return
	}

	repo.Repository = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	var err error
	repo.Owner, err = user_model.GetUserByID(ctx, repo.Repository.OwnerID)
	assert.NoError(t, err)
	repo.RepoLink = repo.Repository.Link()
	repo.Permission, err = access_model.GetUserRepoPermission(ctx, repo.Repository, doer)
	assert.NoError(t, err)
}

// LoadRepoCommit loads a repo's commit into a test context.
func LoadRepoCommit(t *testing.T, ctx gocontext.Context) {
	var repo *context.Repository
	switch ctx := ctx.(type) {
	case *context.Context:
		repo = ctx.Repo
	case *context.APIContext:
		repo = ctx.Repo
	default:
		assert.Fail(t, "context is not *context.Context or *context.APIContext")
		return
	}

	gitRepo, err := git.OpenRepository(ctx, repo.Repository.RepoPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	branch, err := gitRepo.GetHEADBranch()
	assert.NoError(t, err)
	assert.NotNil(t, branch)
	if branch != nil {
		repo.Commit, err = gitRepo.GetBranchCommit(branch.Name)
		assert.NoError(t, err)
	}
}

// LoadUser load a user into a test context.
func LoadUser(t *testing.T, ctx gocontext.Context, userID int64) {
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
	switch ctx := ctx.(type) {
	case *context.Context:
		ctx.Doer = doer
	case *context.APIContext:
		ctx.Doer = doer
	default:
		assert.Fail(t, "context is not *context.Context or *context.APIContext")
		return
	}
}

// LoadGitRepo load a git repo into a test context. Requires that ctx.Repo has
// already been populated.
func LoadGitRepo(t *testing.T, ctx *context.Context) {
	assert.NoError(t, ctx.Repo.Repository.LoadOwner(ctx))
	var err error
	ctx.Repo.GitRepo, err = git.OpenRepository(ctx, ctx.Repo.Repository.RepoPath())
	assert.NoError(t, err)
}

type mockRender struct{}

func (tr *mockRender) TemplateLookup(tmpl string) (templates.TemplateExecutor, error) {
	return nil, nil
}

func (tr *mockRender) HTML(w io.Writer, status int, _ string, _ any) error {
	if resp, ok := w.(http.ResponseWriter); ok {
		resp.WriteHeader(status)
	}
	return nil
}
