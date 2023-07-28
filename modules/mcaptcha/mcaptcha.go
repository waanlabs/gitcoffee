// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcaptcha

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/setting"

	"codeberg.org/gusted/mcaptcha"
)

func Verify(ctx context.Context, token string) (bool, error) {
	valid, err := mcaptcha.Verify(ctx, &mcaptcha.VerifyOpts{
		InstanceURL: setting.Service.McaptchaURL,
		Sitekey:     setting.Service.McaptchaSitekey,
		Secret:      setting.Service.McaptchaSecret,
		Token:       token,
	})
	if err != nil {
		return false, fmt.Errorf("wasn't able to verify mCaptcha: %w", err)
	}
	return valid, nil
}
