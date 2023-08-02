// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

// Uploader uploads all the information of one repository
type Uploader interface {
	MaxBatchInsertSize(tp string) int
	CreateRepo(repo *Repository, opts MigrateOptions) error
	CreateTopics(topic ...string) error
	CreateMilestones(milestones ...*Milestone) error
	CreateReleases(releases ...*Release) error
	SyncTags() error
	CreateLabels(labels ...*Label) error
	CreateIssues(issues ...*Issue) error
	CreateComments(comments ...*Comment) error
	CreatePullRequests(prs ...*PullRequest) error
	CreateReviews(reviews ...*Review) error
	Rollback() error
	Finish() error
	Close()
}
