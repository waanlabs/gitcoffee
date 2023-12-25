// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/label"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

func ToIssue(ctx context.Context, issue *issues_model.Issue) *api.Issue {
	return toIssue(ctx, issue, WebAssetDownloadURL)
}

// ToAPIIssue converts an Issue to API format
// it assumes some fields assigned with values:
// Required - Poster, Labels,
// Optional - Milestone, Assignee, PullRequest
func ToAPIIssue(ctx context.Context, issue *issues_model.Issue) *api.Issue {
	return toIssue(ctx, issue, APIAssetDownloadURL)
}

func toIssue(ctx context.Context, issue *issues_model.Issue, getDownloadURL func(repo *repo_model.Repository, attach *repo_model.Attachment) string) *api.Issue {
	if err := issue.LoadLabels(ctx); err != nil {
		return &api.Issue{}
	}
	if err := issue.LoadPoster(ctx); err != nil {
		return &api.Issue{}
	}
	if err := issue.LoadRepo(ctx); err != nil {
		return &api.Issue{}
	}

	apiIssue := &api.Issue{
		ID:          issue.ID,
		Index:       issue.Index,
		Poster:      ToUser(ctx, issue.Poster, nil),
		Title:       issue.Title,
		Body:        issue.Content,
		Attachments: toAttachments(issue.Repo, issue.Attachments, getDownloadURL),
		Ref:         issue.Ref,
		State:       issue.State(),
		IsLocked:    issue.IsLocked,
		Comments:    issue.NumComments,
		Created:     issue.CreatedUnix.AsTime(),
		Updated:     issue.UpdatedUnix.AsTime(),
		PinOrder:    issue.PinOrder,
	}

	if issue.Repo != nil {
		if err := issue.Repo.LoadOwner(ctx); err != nil {
			return &api.Issue{}
		}
		apiIssue.URL = issue.APIURL()
		apiIssue.HTMLURL = issue.HTMLURL()
		apiIssue.Labels = ToLabelList(issue.Labels, issue.Repo, issue.Repo.Owner)
		apiIssue.Repo = &api.RepositoryMeta{
			ID:       issue.Repo.ID,
			Name:     issue.Repo.Name,
			Owner:    issue.Repo.OwnerName,
			FullName: issue.Repo.FullName(),
		}
	}

	if issue.ClosedUnix != 0 {
		apiIssue.Closed = issue.ClosedUnix.AsTimePtr()
	}

	if err := issue.LoadMilestone(ctx); err != nil {
		return &api.Issue{}
	}
	if issue.Milestone != nil {
		apiIssue.Milestone = ToAPIMilestone(issue.Milestone)
	}

	if err := issue.LoadAssignees(ctx); err != nil {
		return &api.Issue{}
	}
	if len(issue.Assignees) > 0 {
		for _, assignee := range issue.Assignees {
			apiIssue.Assignees = append(apiIssue.Assignees, ToUser(ctx, assignee, nil))
		}
		apiIssue.Assignee = ToUser(ctx, issue.Assignees[0], nil) // For compatibility, we're keeping the first assignee as `apiIssue.Assignee`
	}
	if issue.IsPull {
		if err := issue.LoadPullRequest(ctx); err != nil {
			return &api.Issue{}
		}
		if issue.PullRequest != nil {
			apiIssue.PullRequest = &api.PullRequestMeta{
				HasMerged: issue.PullRequest.HasMerged,
			}
			if issue.PullRequest.HasMerged {
				apiIssue.PullRequest.Merged = issue.PullRequest.MergedUnix.AsTimePtr()
			}
		}
	}
	if issue.DeadlineUnix != 0 {
		apiIssue.Deadline = issue.DeadlineUnix.AsTimePtr()
	}

	return apiIssue
}

// ToIssueList converts an IssueList to API format
func ToIssueList(ctx context.Context, il issues_model.IssueList) []*api.Issue {
	result := make([]*api.Issue, len(il))
	for i := range il {
		result[i] = ToIssue(ctx, il[i])
	}
	return result
}

// ToAPIIssueList converts an IssueList to API format
func ToAPIIssueList(ctx context.Context, il issues_model.IssueList) []*api.Issue {
	result := make([]*api.Issue, len(il))
	for i := range il {
		result[i] = ToAPIIssue(ctx, il[i])
	}
	return result
}

// ToTrackedTime converts TrackedTime to API format
func ToTrackedTime(ctx context.Context, t *issues_model.TrackedTime) (apiT *api.TrackedTime) {
	apiT = &api.TrackedTime{
		ID:      t.ID,
		IssueID: t.IssueID,
		UserID:  t.UserID,
		Time:    t.Time,
		Created: t.Created,
	}
	if t.Issue != nil {
		apiT.Issue = ToAPIIssue(ctx, t.Issue)
	}
	if t.User != nil {
		apiT.UserName = t.User.Name
	}
	return apiT
}

// ToStopWatches convert Stopwatch list to api.StopWatches
func ToStopWatches(sws []*issues_model.Stopwatch) (api.StopWatches, error) {
	result := api.StopWatches(make([]api.StopWatch, 0, len(sws)))

	issueCache := make(map[int64]*issues_model.Issue)
	repoCache := make(map[int64]*repo_model.Repository)
	var (
		issue *issues_model.Issue
		repo  *repo_model.Repository
		ok    bool
		err   error
	)

	for _, sw := range sws {
		issue, ok = issueCache[sw.IssueID]
		if !ok {
			issue, err = issues_model.GetIssueByID(db.DefaultContext, sw.IssueID)
			if err != nil {
				return nil, err
			}
		}
		repo, ok = repoCache[issue.RepoID]
		if !ok {
			repo, err = repo_model.GetRepositoryByID(db.DefaultContext, issue.RepoID)
			if err != nil {
				return nil, err
			}
		}

		result = append(result, api.StopWatch{
			Created:       sw.CreatedUnix.AsTime(),
			Seconds:       sw.Seconds(),
			Duration:      sw.Duration(),
			IssueIndex:    issue.Index,
			IssueTitle:    issue.Title,
			RepoOwnerName: repo.OwnerName,
			RepoName:      repo.Name,
		})
	}
	return result, nil
}

// ToTrackedTimeList converts TrackedTimeList to API format
func ToTrackedTimeList(ctx context.Context, tl issues_model.TrackedTimeList) api.TrackedTimeList {
	result := make([]*api.TrackedTime, 0, len(tl))
	for _, t := range tl {
		result = append(result, ToTrackedTime(ctx, t))
	}
	return result
}

// ToLabel converts Label to API format
func ToLabel(label *issues_model.Label, repo *repo_model.Repository, org *user_model.User) *api.Label {
	result := &api.Label{
		ID:          label.ID,
		Name:        label.Name,
		Exclusive:   label.Exclusive,
		Color:       strings.TrimLeft(label.Color, "#"),
		Description: label.Description,
		IsArchived:  label.IsArchived(),
	}

	// calculate URL
	if label.BelongsToRepo() && repo != nil {
		if repo != nil {
			result.URL = fmt.Sprintf("%s/labels/%d", repo.APIURL(), label.ID)
		} else {
			log.Error("ToLabel did not get repo to calculate url for label with id '%d'", label.ID)
		}
	} else { // BelongsToOrg
		if org != nil {
			result.URL = fmt.Sprintf("%sapi/v1/orgs/%s/labels/%d", setting.AppURL, url.PathEscape(org.Name), label.ID)
		} else {
			log.Error("ToLabel did not get org to calculate url for label with id '%d'", label.ID)
		}
	}

	return result
}

// ToLabelList converts list of Label to API format
func ToLabelList(labels []*issues_model.Label, repo *repo_model.Repository, org *user_model.User) []*api.Label {
	result := make([]*api.Label, len(labels))
	for i := range labels {
		result[i] = ToLabel(labels[i], repo, org)
	}
	return result
}

// ToAPIMilestone converts Milestone into API Format
func ToAPIMilestone(m *issues_model.Milestone) *api.Milestone {
	apiMilestone := &api.Milestone{
		ID:           m.ID,
		State:        m.State(),
		Title:        m.Name,
		Description:  m.Content,
		OpenIssues:   m.NumOpenIssues,
		ClosedIssues: m.NumClosedIssues,
		Created:      m.CreatedUnix.AsTime(),
		Updated:      m.UpdatedUnix.AsTimePtr(),
	}
	if m.IsClosed {
		apiMilestone.Closed = m.ClosedDateUnix.AsTimePtr()
	}
	if m.DeadlineUnix.Year() < 9999 {
		apiMilestone.Deadline = m.DeadlineUnix.AsTimePtr()
	}
	return apiMilestone
}

// ToLabelTemplate converts Label to API format
func ToLabelTemplate(label *label.Label) *api.LabelTemplate {
	result := &api.LabelTemplate{
		Name:        label.Name,
		Exclusive:   label.Exclusive,
		Color:       strings.TrimLeft(label.Color, "#"),
		Description: label.Description,
	}

	return result
}

// ToLabelTemplateList converts list of Label to API format
func ToLabelTemplateList(labels []*label.Label) []*api.LabelTemplate {
	result := make([]*api.LabelTemplate, len(labels))
	for i := range labels {
		result[i] = ToLabelTemplate(labels[i])
	}
	return result
}
