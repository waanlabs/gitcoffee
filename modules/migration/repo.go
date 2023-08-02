// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

// Repository defines a standard repository information
type Repository struct {
	Name          string
	Owner         string
	IsPrivate     bool `yaml:"is_private"`
	IsMirror      bool `yaml:"is_mirror"`
	Description   string
	CloneURL      string `yaml:"clone_url"` // SECURITY: This must be checked to ensure that is safe to be used
	OriginalURL   string `yaml:"original_url"`
	DefaultBranch string
}
