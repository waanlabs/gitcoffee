// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/web/repo"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/model"
)

const (
	tplListActions base.TplName = "repo/actions/list"
	tplViewActions base.TplName = "repo/actions/view"
)

type Workflow struct {
	Entry  git.TreeEntry
	ErrMsg string
}

// MustEnableActions check if actions are enabled in settings
func MustEnableActions(ctx *context.Context) {
	if !setting.Actions.Enabled {
		ctx.NotFound("MustEnableActions", nil)
		return
	}

	if unit.TypeActions.UnitGlobalDisabled() {
		ctx.NotFound("MustEnableActions", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.CanRead(unit.TypeActions) {
			ctx.NotFound("MustEnableActions", nil)
			return
		}
	}
}

func List(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsActions"] = true

	var workflows []Workflow
	if empty, err := ctx.Repo.GitRepo.IsEmpty(); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	} else if !empty {
		commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		entries, err := actions.ListWorkflows(commit)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}

		// Get all runner labels
		opts := actions_model.FindRunnerOptions{
			RepoID:        ctx.Repo.Repository.ID,
			WithAvailable: true,
		}
		runners, err := actions_model.FindRunners(ctx, opts)
		if err != nil {
			ctx.ServerError("FindRunners", err)
			return
		}
		allRunnerLabels := make(container.Set[string])
		for _, r := range runners {
			allRunnerLabels.AddMultiple(r.AgentLabels...)
		}

		workflows = make([]Workflow, 0, len(entries))
		for _, entry := range entries {
			workflow := Workflow{Entry: *entry}
			content, err := actions.GetContentFromEntry(entry)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, err.Error())
				return
			}
			wf, err := model.ReadWorkflow(bytes.NewReader(content))
			if err != nil {
				workflow.ErrMsg = ctx.Locale.Tr("actions.runs.invalid_workflow_helper", err.Error())
				workflows = append(workflows, workflow)
				continue
			}
			// Check whether have matching runner
			for _, j := range wf.Jobs {
				runsOnList := j.RunsOn()
				for _, ro := range runsOnList {
					if strings.Contains(ro, "${{") {
						// Skip if it contains expressions.
						// The expressions could be very complex and could not be evaluated here,
						// so just skip it, it's OK since it's just a tooltip message.
						continue
					}
					if !allRunnerLabels.Contains(ro) {
						workflow.ErrMsg = ctx.Locale.Tr("actions.runs.no_matching_runner_helper", ro)
						break
					}
				}
				if workflow.ErrMsg != "" {
					break
				}
			}
			workflows = append(workflows, workflow)
		}
	}
	ctx.Data["workflows"] = workflows
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	workflow := ctx.FormString("workflow")
	actorID := ctx.FormInt64("actor")
	status := ctx.FormInt("status")
	ctx.Data["CurWorkflow"] = workflow

	actionsConfig := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	ctx.Data["ActionsConfig"] = actionsConfig

	if len(workflow) > 0 && ctx.Repo.IsAdmin() {
		ctx.Data["AllowDisableOrEnableWorkflow"] = true
		ctx.Data["CurWorkflowDisabled"] = actionsConfig.IsWorkflowDisabled(workflow)
	}

	// if status or actor query param is not given to frontend href, (href="/<repoLink>/actions")
	// they will be 0 by default, which indicates get all status or actors
	ctx.Data["CurActor"] = actorID
	ctx.Data["CurStatus"] = status
	if actorID > 0 || status > int(actions_model.StatusUnknown) {
		ctx.Data["IsFiltered"] = true
	}

	opts := actions_model.FindRunOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		RepoID:        ctx.Repo.Repository.ID,
		WorkflowID:    workflow,
		TriggerUserID: actorID,
	}

	// if status is not StatusUnknown, it means user has selected a status filter
	if actions_model.Status(status) != actions_model.StatusUnknown {
		opts.Status = []actions_model.Status{actions_model.Status(status)}
	}

	runs, total, err := actions_model.FindRuns(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	for _, run := range runs {
		run.Repo = ctx.Repo.Repository
	}

	if err := runs.LoadTriggerUser(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data["Runs"] = runs

	actors, err := actions_model.GetActors(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Data["Actors"] = repo.MakeSelfOnTop(ctx.Doer, actors)

	ctx.Data["StatusInfoList"] = actions_model.GetStatusInfoList(ctx)

	pager := context.NewPagination(int(total), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParamString("workflow", workflow)
	pager.AddParamString("actor", fmt.Sprint(actorID))
	pager.AddParamString("status", fmt.Sprint(status))
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplListActions)
}
