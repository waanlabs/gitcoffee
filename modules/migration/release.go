// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"io"
	"time"
)

// ReleaseAsset represents a release asset
type ReleaseAsset struct {
	ID            int64
	Name          string
	ContentType   *string `yaml:"content_type"`
	Size          *int
	DownloadCount *int `yaml:"download_count"`
	Created       time.Time
	Updated       time.Time

	DownloadURL *string `yaml:"download_url"` // SECURITY: It is the responsibility of downloader to make sure this is safe
	// if DownloadURL is nil, the function should be invoked
	DownloadFunc func() (io.ReadCloser, error) `yaml:"-"` // SECURITY: It is the responsibility of downloader to make sure this is safe
}

// Release represents a release
type Release struct {
	TagName         string `yaml:"tag_name"`         // SECURITY: This must pass git.IsValidRefPattern
	TargetCommitish string `yaml:"target_commitish"` // SECURITY: This must pass git.IsValidRefPattern
	Name            string
	Body            string
	Draft           bool
	Prerelease      bool
	PublisherID     int64  `yaml:"publisher_id"`
	PublisherName   string `yaml:"publisher_name"`
	PublisherEmail  string `yaml:"publisher_email"`
	Assets          []*ReleaseAsset
	Created         time.Time
	Published       time.Time
}

// GetExternalName ExternalUserMigrated interface
func (r *Release) GetExternalName() string { return r.PublisherName }

// GetExternalID ExternalUserMigrated interface
func (r *Release) GetExternalID() int64 { return r.PublisherID }
