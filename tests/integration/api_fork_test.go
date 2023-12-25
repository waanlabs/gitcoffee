// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
)

func TestCreateForkNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{})
	MakeRequest(t, req, http.StatusUnauthorized)
}
