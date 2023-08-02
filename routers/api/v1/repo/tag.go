// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
	releaseservice "code.gitea.io/gitea/services/release"
)

// ListTags list all the tags of a repository
func ListTags(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/tags repository repoListTags
	// ---
	// summary: List a repository's tags
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TagList"

	listOpts := utils.GetListOptions(ctx)

	tags, total, err := ctx.Repo.GitRepo.GetTagInfos(listOpts.Page, listOpts.PageSize)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTags", err)
		return
	}

	apiTags := make([]*api.Tag, len(tags))
	for i := range tags {
		apiTags[i] = convert.ToTag(ctx.Repo.Repository, tags[i])
	}

	ctx.SetTotalCountHeader(int64(total))
	ctx.JSON(http.StatusOK, &apiTags)
}

// GetAnnotatedTag get the tag of a repository.
func GetAnnotatedTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/tags/{sha} repository GetAnnotatedTag
	// ---
	// summary: Gets the tag object of an annotated tag (not lightweight tags)
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: sha
	//   in: path
	//   description: sha of the tag. The Git tags API only supports annotated tag objects, not lightweight tags.
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AnnotatedTag"
	//   "400":
	//     "$ref": "#/responses/error"

	sha := ctx.Params("sha")
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "", "SHA not provided")
		return
	}

	if tag, err := ctx.Repo.GitRepo.GetAnnotatedTag(sha); err != nil {
		ctx.Error(http.StatusBadRequest, "GetAnnotatedTag", err)
	} else {
		commit, err := tag.Commit(ctx.Repo.GitRepo)
		if err != nil {
			ctx.Error(http.StatusBadRequest, "GetAnnotatedTag", err)
		}
		ctx.JSON(http.StatusOK, convert.ToAnnotatedTag(ctx, ctx.Repo.Repository, tag, commit))
	}
}

// GetTag get the tag of a repository
func GetTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/tags/{tag} repository repoGetTag
	// ---
	// summary: Get the tag of a repository by tag name
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: tag
	//   in: path
	//   description: name of tag
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Tag"
	//   "404":
	//     "$ref": "#/responses/notFound"
	tagName := ctx.Params("*")

	tag, err := ctx.Repo.GitRepo.GetTag(tagName)
	if err != nil {
		ctx.NotFound(tagName)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToTag(ctx.Repo.Repository, tag))
}

// CreateTag create a new git tag in a repository
func CreateTag(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/tags repository repoCreateTag
	// ---
	// summary: Create a new git tag in a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateTagOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Tag"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "405":
	//     "$ref": "#/responses/empty"
	//   "409":
	//     "$ref": "#/responses/conflict"
	form := web.GetForm(ctx).(*api.CreateTagOption)

	// If target is not provided use default branch
	if len(form.Target) == 0 {
		form.Target = ctx.Repo.Repository.DefaultBranch
	}

	commit, err := ctx.Repo.GitRepo.GetCommit(form.Target)
	if err != nil {
		ctx.Error(http.StatusNotFound, "target not found", fmt.Errorf("target not found: %w", err))
		return
	}

	if err := releaseservice.CreateNewTag(ctx, ctx.Doer, ctx.Repo.Repository, commit.ID.String(), form.TagName, form.Message); err != nil {
		if models.IsErrTagAlreadyExists(err) {
			ctx.Error(http.StatusConflict, "tag exist", err)
			return
		}
		if models.IsErrProtectedTagName(err) {
			ctx.Error(http.StatusMethodNotAllowed, "CreateNewTag", "user not allowed to create protected tag")
			return
		}

		ctx.InternalServerError(err)
		return
	}

	tag, err := ctx.Repo.GitRepo.GetTag(form.TagName)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToTag(ctx.Repo.Repository, tag))
}

// DeleteTag delete a specific tag of in a repository by name
func DeleteTag(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/tags/{tag} repository repoDeleteTag
	// ---
	// summary: Delete a repository's tag by name
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: tag
	//   in: path
	//   description: name of tag to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "405":
	//     "$ref": "#/responses/empty"
	//   "409":
	//     "$ref": "#/responses/conflict"
	tagName := ctx.Params("*")

	tag, err := repo_model.GetRelease(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRelease", err)
		return
	}

	if !tag.IsTag {
		ctx.Error(http.StatusConflict, "IsTag", errors.New("a tag attached to a release cannot be deleted directly"))
		return
	}

	if err = releaseservice.DeleteReleaseByID(ctx, tag.ID, ctx.Doer, true); err != nil {
		if models.IsErrProtectedTagName(err) {
			ctx.Error(http.StatusMethodNotAllowed, "delTag", "user not allowed to delete protected tag")
			return
		}
		ctx.Error(http.StatusInternalServerError, "DeleteReleaseByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
