// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	"xorm.io/builder"
)

// CountOrphanedObjects count subjects with have no existing refobject anymore
func CountOrphanedObjects(ctx context.Context, subject, refobject, joinCond string) (int64, error) {
	return GetEngine(ctx).
		Table("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"}).
		Select("COUNT(`" + subject + "`.`id`)").
		Count()
}

// DeleteOrphanedObjects delete subjects with have no existing refobject anymore
func DeleteOrphanedObjects(ctx context.Context, subject, refobject, joinCond string) error {
	subQuery := builder.Select("`"+subject+"`.id").
		From("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"})
	b := builder.Delete(builder.In("id", subQuery)).From("`" + subject + "`")
	_, err := GetEngine(ctx).Exec(b)
	return err
}
