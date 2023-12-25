// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
)

// Markup render markup document to HTML
func Markup(ctx *context.Context) {
	form := web.GetForm(ctx).(*api.MarkupOption)
	common.RenderMarkup(ctx.Base, ctx.Repo, form.Mode, form.Text, form.Context, form.FilePath, form.Wiki)
}
