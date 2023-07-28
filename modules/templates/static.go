// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package templates

import (
	"time"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/timeutil"
)

// GlobalModTime provide a global mod time for embedded asset files
func GlobalModTime(filename string) time.Time {
	return timeutil.GetExecutableModTime()
}

func BuiltinAssets() *assetfs.Layer {
	return assetfs.Bindata("builtin(bindata)", Assets)
}
