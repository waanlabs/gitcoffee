// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/tests"
)

func TestOrgProjectAccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// disable repo project unit
	unit_model.DisabledRepoUnits = []unit_model.Type{unit_model.TypeProjects}

	// repo project, 404
	req := NewRequest(t, "GET", "/user2/repo1/projects")
	MakeRequest(t, req, http.StatusNotFound)

	// user project, 200
	req = NewRequest(t, "GET", "/user2/-/projects")
	MakeRequest(t, req, http.StatusOK)

	// org project, 200
	req = NewRequest(t, "GET", "/user3/-/projects")
	MakeRequest(t, req, http.StatusOK)

	// change the org's visibility to private
	session := loginUser(t, "user2")
	req = NewRequestWithValues(t, "POST", "/org/user3/settings", map[string]string{
		"_csrf":      GetCSRF(t, session, "/user3/-/projects"),
		"name":       "user3",
		"visibility": "2",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// user4 can still access the org's project because its team(team1) has the permission
	session = loginUser(t, "user4")
	req = NewRequest(t, "GET", "/user3/-/projects")
	session.MakeRequest(t, req, http.StatusOK)

	// disable team1's project unit
	session = loginUser(t, "user2")
	req = NewRequestWithValues(t, "POST", "/org/user3/teams/team1/edit", map[string]string{
		"_csrf":       GetCSRF(t, session, "/user3/-/projects"),
		"team_name":   "team1",
		"repo_access": "specific",
		"permission":  "read",
		"unit_8":      "0",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// user4 can no longer access the org's project
	session = loginUser(t, "user4")
	req = NewRequest(t, "GET", "/user3/-/projects")
	session.MakeRequest(t, req, http.StatusNotFound)
}
