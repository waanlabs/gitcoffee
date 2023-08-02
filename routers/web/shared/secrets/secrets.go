// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"code.gitea.io/gitea/models/db"
	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/shared/actions"
	"code.gitea.io/gitea/services/forms"
)

func SetSecretsContext(ctx *context.Context, ownerID, repoID int64) {
	secrets, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{OwnerID: ownerID, RepoID: repoID})
	if err != nil {
		ctx.ServerError("FindSecrets", err)
		return
	}

	ctx.Data["Secrets"] = secrets
}

func PerformSecretsPost(ctx *context.Context, ownerID, repoID int64, redirectURL string) {
	form := web.GetForm(ctx).(*forms.AddSecretForm)

	if err := actions.NameRegexMatch(form.Name); err != nil {
		ctx.JSONError(ctx.Tr("secrets.creation.failed"))
		return
	}

	s, err := secret_model.InsertEncryptedSecret(ctx, ownerID, repoID, form.Name, actions.ReserveLineBreakForTextarea(form.Data))
	if err != nil {
		log.Error("InsertEncryptedSecret: %v", err)
		ctx.JSONError(ctx.Tr("secrets.creation.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.creation.success", s.Name))
	ctx.JSONRedirect(redirectURL)
}

func PerformSecretsDelete(ctx *context.Context, ownerID, repoID int64, redirectURL string) {
	id := ctx.FormInt64("id")

	if _, err := db.DeleteByBean(ctx, &secret_model.Secret{ID: id, OwnerID: ownerID, RepoID: repoID}); err != nil {
		log.Error("Delete secret %d failed: %v", id, err)
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("secrets.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}
