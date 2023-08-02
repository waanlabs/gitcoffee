// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/label"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// ListLabels list all the labels of a repository
func ListLabels(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/labels issue issueListLabels
	// ---
	// summary: Get all of a repository's labels
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
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"

	labels, err := issues_model.GetLabelsByRepoID(ctx, ctx.Repo.Repository.ID, ctx.FormString("sort"), utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByRepoID", err)
		return
	}

	count, err := issues_model.CountLabelsByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, convert.ToLabelList(labels, ctx.Repo.Repository, nil))
}

// GetLabel get label by repository and label id
func GetLabel(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/labels/{id} issue issueGetLabel
	// ---
	// summary: Get a single label
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
	// - name: id
	//   in: path
	//   description: id of the label to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Label"

	var (
		l   *issues_model.Label
		err error
	)
	strID := ctx.Params(":id")
	if intID, err2 := strconv.ParseInt(strID, 10, 64); err2 != nil {
		l, err = issues_model.GetLabelInRepoByName(ctx, ctx.Repo.Repository.ID, strID)
	} else {
		l, err = issues_model.GetLabelInRepoByID(ctx, ctx.Repo.Repository.ID, intID)
	}
	if err != nil {
		if issues_model.IsErrRepoLabelNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByRepoID", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabel(l, ctx.Repo.Repository, nil))
}

// CreateLabel create a label for a repository
func CreateLabel(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/labels issue issueCreateLabel
	// ---
	// summary: Create a label
	// consumes:
	// - application/json
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
	//     "$ref": "#/definitions/CreateLabelOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Label"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateLabelOption)

	color, err := label.NormalizeColor(form.Color)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "StringToColor", err)
		return
	}
	form.Color = color

	l := &issues_model.Label{
		Name:        form.Name,
		Exclusive:   form.Exclusive,
		Color:       form.Color,
		RepoID:      ctx.Repo.Repository.ID,
		Description: form.Description,
	}
	if err := issues_model.NewLabel(ctx, l); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewLabel", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToLabel(l, ctx.Repo.Repository, nil))
}

// EditLabel modify a label for a repository
func EditLabel(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/labels/{id} issue issueEditLabel
	// ---
	// summary: Update a label
	// consumes:
	// - application/json
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
	// - name: id
	//   in: path
	//   description: id of the label to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditLabelOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Label"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.EditLabelOption)
	l, err := issues_model.GetLabelInRepoByID(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if issues_model.IsErrRepoLabelNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByRepoID", err)
		}
		return
	}

	if form.Name != nil {
		l.Name = *form.Name
	}
	if form.Exclusive != nil {
		l.Exclusive = *form.Exclusive
	}
	if form.Color != nil {
		color, err := label.NormalizeColor(*form.Color)
		if err != nil {
			ctx.Error(http.StatusUnprocessableEntity, "StringToColor", err)
			return
		}
		l.Color = color
	}
	if form.Description != nil {
		l.Description = *form.Description
	}
	if err := issues_model.UpdateLabel(l); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateLabel", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabel(l, ctx.Repo.Repository, nil))
}

// DeleteLabel delete a label for a repository
func DeleteLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/labels/{id} issue issueDeleteLabel
	// ---
	// summary: Delete a label
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
	// - name: id
	//   in: path
	//   description: id of the label to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	if err := issues_model.DeleteLabel(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id")); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteLabel", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
