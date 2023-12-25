// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestDownloadByID(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo1/raw/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
}

func TestDownloadByIDForSVGUsesSecureHeaders(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo2/raw/blob/6395b68e1feebb1e4c657b4f9f6ba2676a283c0b")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", resp.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "image/svg+xml", resp.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", resp.Header().Get("X-Content-Type-Options"))
}

func TestDownloadByIDMedia(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo1/media/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
}

func TestDownloadByIDMediaForSVGUsesSecureHeaders(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo2/media/blob/6395b68e1feebb1e4c657b4f9f6ba2676a283c0b")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", resp.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "image/svg+xml", resp.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", resp.Header().Get("X-Content-Type-Options"))
}

func TestDownloadRawTextFileWithoutMimeTypeMapping(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo2/raw/branch/master/test.xml")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
}

func TestDownloadRawTextFileWithMimeTypeMapping(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setting.MimeTypeMap.Map[".xml"] = "text/xml"
	setting.MimeTypeMap.Enabled = true

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo2/raw/branch/master/test.xml")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "text/xml; charset=utf-8", resp.Header().Get("Content-Type"))

	delete(setting.MimeTypeMap.Map, ".xml")
	setting.MimeTypeMap.Enabled = false
}
