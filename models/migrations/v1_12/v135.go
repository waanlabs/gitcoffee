// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddOrgIDLabelColumn(x *xorm.Engine) error {
	type Label struct {
		OrgID int64 `xorm:"INDEX"`
	}

	if err := x.Sync2(new(Label)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
