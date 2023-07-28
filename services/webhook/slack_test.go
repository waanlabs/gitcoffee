// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(SlackPayload)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>:<http://localhost:3000/test/repo/src/branch/test|test>] branch created by user1", pl.(*SlackPayload).Text)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(SlackPayload)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>:test] branch deleted by user1", pl.(*SlackPayload).Text)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(SlackPayload)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "<http://localhost:3000/test/repo2|test/repo2> is forked to <http://localhost:3000/test/repo|test/repo>", pl.(*SlackPayload).Text)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(SlackPayload)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>:<http://localhost:3000/test/repo/src/branch/test|test>] 2 new commits pushed by user1", pl.(*SlackPayload).Text)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(SlackPayload)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue opened: <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue closed: <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(SlackPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] New comment on issue <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(SlackPayload)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Pull request opened: <http://localhost:3000/test/repo/pulls/12|#12 Fix bug> by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(SlackPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] New comment on pull request <http://localhost:3000/test/repo/pulls/12|#12 Fix bug> by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(SlackPayload)
		pl, err := d.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Pull request review approved: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(SlackPayload)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Repository created by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		d := new(SlackPayload)
		p.Action = api.HookWikiCreated
		pl, err := d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] New wiki page '<http://localhost:3000/test/repo/wiki/index|index>' (Wiki change comment) by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)

		p.Action = api.HookWikiEdited
		pl, err = d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Wiki page '<http://localhost:3000/test/repo/wiki/index|index>' edited (Wiki change comment) by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)

		p.Action = api.HookWikiDeleted
		pl, err = d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Wiki page '<http://localhost:3000/test/repo/wiki/index|index>' deleted by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(SlackPayload)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &SlackPayload{}, pl)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Release created: <http://localhost:3000/test/repo/releases/tag/v1.0|v1.0> by <https://try.gitea.io/user1|user1>", pl.(*SlackPayload).Text)
	})
}

func TestSlackJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(SlackPayload).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &SlackPayload{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}

func TestIsValidSlackChannel(t *testing.T) {
	tt := []struct {
		channelName string
		expected    bool
	}{
		{"gitea", true},
		{"#gitea", true},
		{"  ", false},
		{"#", false},
		{" #", false},
		{"gitea   ", false},
		{"  gitea", false},
	}

	for _, v := range tt {
		assert.Equal(t, v.expected, IsValidSlackChannel(v.channelName))
	}
}
