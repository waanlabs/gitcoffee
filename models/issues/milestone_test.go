// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"sort"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func TestMilestone_State(t *testing.T) {
	assert.Equal(t, api.StateOpen, (&issues_model.Milestone{IsClosed: false}).State())
	assert.Equal(t, api.StateClosed, (&issues_model.Milestone{IsClosed: true}).State())
}

func TestGetMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	milestone, err := issues_model.GetMilestoneByRepoID(db.DefaultContext, 1, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, milestone.ID)
	assert.EqualValues(t, 1, milestone.RepoID)

	_, err = issues_model.GetMilestoneByRepoID(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID)
	assert.True(t, issues_model.IsErrMilestoneNotExist(err))
}

func TestGetMilestonesByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64, state api.StateType) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		milestones, _, err := issues_model.GetMilestones(issues_model.GetMilestonesOption{
			RepoID: repo.ID,
			State:  state,
		})
		assert.NoError(t, err)

		var n int

		switch state {
		case api.StateClosed:
			n = repo.NumClosedMilestones

		case api.StateAll:
			n = repo.NumMilestones

		case api.StateOpen:
			fallthrough

		default:
			n = repo.NumOpenMilestones
		}

		assert.Len(t, milestones, n)
		for _, milestone := range milestones {
			assert.EqualValues(t, repoID, milestone.RepoID)
		}
	}
	test(1, api.StateOpen)
	test(1, api.StateAll)
	test(1, api.StateClosed)
	test(2, api.StateOpen)
	test(2, api.StateAll)
	test(2, api.StateClosed)
	test(3, api.StateOpen)
	test(3, api.StateClosed)
	test(3, api.StateAll)

	milestones, _, err := issues_model.GetMilestones(issues_model.GetMilestonesOption{
		RepoID: unittest.NonexistentID,
		State:  api.StateOpen,
	})
	assert.NoError(t, err)
	assert.Len(t, milestones, 0)
}

func TestGetMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	test := func(sortType string, sortCond func(*issues_model.Milestone) int) {
		for _, page := range []int{0, 1} {
			milestones, _, err := issues_model.GetMilestones(issues_model.GetMilestonesOption{
				ListOptions: db.ListOptions{
					Page:     page,
					PageSize: setting.UI.IssuePagingNum,
				},
				RepoID:   repo.ID,
				State:    api.StateOpen,
				SortType: sortType,
			})
			assert.NoError(t, err)
			assert.Len(t, milestones, repo.NumMilestones-repo.NumClosedMilestones)
			values := make([]int, len(milestones))
			for i, milestone := range milestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))

			milestones, _, err = issues_model.GetMilestones(issues_model.GetMilestonesOption{
				ListOptions: db.ListOptions{
					Page:     page,
					PageSize: setting.UI.IssuePagingNum,
				},
				RepoID:   repo.ID,
				State:    api.StateClosed,
				Name:     "",
				SortType: sortType,
			})
			assert.NoError(t, err)
			assert.Len(t, milestones, repo.NumClosedMilestones)
			values = make([]int, len(milestones))
			for i, milestone := range milestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))
		}
	}
	test("furthestduedate", func(milestone *issues_model.Milestone) int {
		return -int(milestone.DeadlineUnix)
	})
	test("leastcomplete", func(milestone *issues_model.Milestone) int {
		return milestone.Completeness
	})
	test("mostcomplete", func(milestone *issues_model.Milestone) int {
		return -milestone.Completeness
	})
	test("leastissues", func(milestone *issues_model.Milestone) int {
		return milestone.NumIssues
	})
	test("mostissues", func(milestone *issues_model.Milestone) int {
		return -milestone.NumIssues
	})
	test("soonestduedate", func(milestone *issues_model.Milestone) int {
		return int(milestone.DeadlineUnix)
	})
}

func TestCountRepoMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		count, err := issues_model.CountMilestones(db.DefaultContext, issues_model.GetMilestonesOption{
			RepoID: repoID,
			State:  api.StateAll,
		})
		assert.NoError(t, err)
		assert.EqualValues(t, repo.NumMilestones, count)
	}
	test(1)
	test(2)
	test(3)

	count, err := issues_model.CountMilestones(db.DefaultContext, issues_model.GetMilestonesOption{
		RepoID: unittest.NonexistentID,
		State:  api.StateAll,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestCountRepoClosedMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		count, err := issues_model.CountMilestones(db.DefaultContext, issues_model.GetMilestonesOption{
			RepoID: repoID,
			State:  api.StateClosed,
		})
		assert.NoError(t, err)
		assert.EqualValues(t, repo.NumClosedMilestones, count)
	}
	test(1)
	test(2)
	test(3)

	count, err := issues_model.CountMilestones(db.DefaultContext, issues_model.GetMilestonesOption{
		RepoID: unittest.NonexistentID,
		State:  api.StateClosed,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestCountMilestonesByRepoIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestonesCount := func(repoID int64) (int, int) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		return repo.NumOpenMilestones, repo.NumClosedMilestones
	}
	repo1OpenCount, repo1ClosedCount := milestonesCount(1)
	repo2OpenCount, repo2ClosedCount := milestonesCount(2)

	openCounts, err := issues_model.CountMilestonesByRepoCond(builder.In("repo_id", []int64{1, 2}), false)
	assert.NoError(t, err)
	assert.EqualValues(t, repo1OpenCount, openCounts[1])
	assert.EqualValues(t, repo2OpenCount, openCounts[2])

	closedCounts, err := issues_model.CountMilestonesByRepoCond(builder.In("repo_id", []int64{1, 2}), true)
	assert.NoError(t, err)
	assert.EqualValues(t, repo1ClosedCount, closedCounts[1])
	assert.EqualValues(t, repo2ClosedCount, closedCounts[2])
}

func TestGetMilestonesByRepoIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	test := func(sortType string, sortCond func(*issues_model.Milestone) int) {
		for _, page := range []int{0, 1} {
			openMilestones, err := issues_model.GetMilestonesByRepoIDs([]int64{repo1.ID, repo2.ID}, page, false, sortType)
			assert.NoError(t, err)
			assert.Len(t, openMilestones, repo1.NumOpenMilestones+repo2.NumOpenMilestones)
			values := make([]int, len(openMilestones))
			for i, milestone := range openMilestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))

			closedMilestones, err := issues_model.GetMilestonesByRepoIDs([]int64{repo1.ID, repo2.ID}, page, true, sortType)
			assert.NoError(t, err)
			assert.Len(t, closedMilestones, repo1.NumClosedMilestones+repo2.NumClosedMilestones)
			values = make([]int, len(closedMilestones))
			for i, milestone := range closedMilestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))
		}
	}
	test("furthestduedate", func(milestone *issues_model.Milestone) int {
		return -int(milestone.DeadlineUnix)
	})
	test("leastcomplete", func(milestone *issues_model.Milestone) int {
		return milestone.Completeness
	})
	test("mostcomplete", func(milestone *issues_model.Milestone) int {
		return -milestone.Completeness
	})
	test("leastissues", func(milestone *issues_model.Milestone) int {
		return milestone.NumIssues
	})
	test("mostissues", func(milestone *issues_model.Milestone) int {
		return -milestone.NumIssues
	})
	test("soonestduedate", func(milestone *issues_model.Milestone) int {
		return int(milestone.DeadlineUnix)
	})
}

func TestGetMilestonesStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		stats, err := issues_model.GetMilestonesStatsByRepoCond(builder.And(builder.Eq{"repo_id": repoID}))
		assert.NoError(t, err)
		assert.EqualValues(t, repo.NumMilestones-repo.NumClosedMilestones, stats.OpenCount)
		assert.EqualValues(t, repo.NumClosedMilestones, stats.ClosedCount)
	}
	test(1)
	test(2)
	test(3)

	stats, err := issues_model.GetMilestonesStatsByRepoCond(builder.And(builder.Eq{"repo_id": unittest.NonexistentID}))
	assert.NoError(t, err)
	assert.EqualValues(t, 0, stats.OpenCount)
	assert.EqualValues(t, 0, stats.ClosedCount)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	milestoneStats, err := issues_model.GetMilestonesStatsByRepoCond(builder.In("repo_id", []int64{repo1.ID, repo2.ID}))
	assert.NoError(t, err)
	assert.EqualValues(t, repo1.NumOpenMilestones+repo2.NumOpenMilestones, milestoneStats.OpenCount)
	assert.EqualValues(t, repo1.NumClosedMilestones+repo2.NumClosedMilestones, milestoneStats.ClosedCount)
}

func TestNewMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := &issues_model.Milestone{
		RepoID:  1,
		Name:    "milestoneName",
		Content: "milestoneContent",
	}

	assert.NoError(t, issues_model.NewMilestone(milestone))
	unittest.AssertExistsAndLoadBean(t, milestone)
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})
}

func TestChangeMilestoneStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})

	assert.NoError(t, issues_model.ChangeMilestoneStatus(milestone, true))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}, "is_closed=1")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})

	assert.NoError(t, issues_model.ChangeMilestoneStatus(milestone, false))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}, "is_closed=0")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})
}

func TestDeleteMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.NoError(t, issues_model.DeleteMilestoneByRepoID(1, 1))
	unittest.AssertNotExistsBean(t, &issues_model.Milestone{ID: 1})
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: 1})

	assert.NoError(t, issues_model.DeleteMilestoneByRepoID(unittest.NonexistentID, unittest.NonexistentID))
}

func TestUpdateMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})
	milestone.Name = " newMilestoneName  "
	milestone.Content = "newMilestoneContent"
	assert.NoError(t, issues_model.UpdateMilestone(milestone, milestone.IsClosed))
	milestone = unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})
	assert.EqualValues(t, "newMilestoneName", milestone.Name)
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})
}

func TestUpdateMilestoneCounters(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{MilestoneID: 1},
		"is_closed=0")

	issue.IsClosed = true
	issue.ClosedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})

	issue.IsClosed = false
	issue.ClosedUnix = 0
	_, err = db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})
}
