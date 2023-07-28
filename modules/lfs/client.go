// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// DownloadCallback gets called for every requested LFS object to process its content
type DownloadCallback func(p Pointer, content io.ReadCloser, objectError error) error

// UploadCallback gets called for every requested LFS object to provide its content
type UploadCallback func(p Pointer, objectError error) (io.ReadCloser, error)

// Client is used to communicate with a LFS source
type Client interface {
	BatchSize() int
	Download(ctx context.Context, objects []Pointer, callback DownloadCallback) error
	Upload(ctx context.Context, objects []Pointer, callback UploadCallback) error
}

// NewClient creates a LFS client
func NewClient(endpoint *url.URL, httpTransport *http.Transport) Client {
	if endpoint.Scheme == "file" {
		return newFilesystemClient(endpoint)
	}
	return newHTTPClient(endpoint, httpTransport)
}
