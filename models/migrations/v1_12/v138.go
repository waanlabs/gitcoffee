// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddResolveDoerIDCommentColumn(x *xorm.Engine) error {
	type Comment struct {
		ResolveDoerID int64
	}

	if err := x.Sync2(new(Comment)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
