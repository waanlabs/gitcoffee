// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// Actions settings
var (
	Actions = struct {
		LogStorage        *Storage // how the created logs should be stored
		ArtifactStorage   *Storage // how the created artifacts should be stored
		Enabled           bool
		DefaultActionsURL defaultActionsURL `ini:"DEFAULT_ACTIONS_URL"`
	}{
		Enabled:           false,
		DefaultActionsURL: defaultActionsURLGitHub,
	}
)

type defaultActionsURL string

func (url defaultActionsURL) URL() string {
	switch url {
	case defaultActionsURLGitHub:
		return "https://github.com"
	case defaultActionsURLSelf:
		return strings.TrimSuffix(AppURL, "/")
	default:
		// This should never happen, but just in case, use GitHub as fallback
		return "https://github.com"
	}
}

const (
	defaultActionsURLGitHub = "github" // https://github.com
	defaultActionsURLSelf   = "self"   // the root URL of the self-hosted Gitea instance
	// DefaultActionsURL only supports GitHub and the self-hosted Gitea.
	// It's intentionally not supported more, so please be cautious before adding more like "gitea" or "gitlab".
	// If you get some trouble with `uses: username/action_name@version` in your workflow,
	// please consider to use `uses: https://the_url_you_want_to_use/username/action_name@version` instead.
)

func loadActionsFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("actions")
	err := sec.MapTo(&Actions)
	if err != nil {
		return fmt.Errorf("failed to map Actions settings: %v", err)
	}

	if urls := string(Actions.DefaultActionsURL); urls != defaultActionsURLGitHub && urls != defaultActionsURLSelf {
		url := strings.Split(urls, ",")[0]
		if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
			log.Error("[actions] DEFAULT_ACTIONS_URL does not support %q as custom URL any longer, fallback to %q",
				urls,
				defaultActionsURLGitHub,
			)
			Actions.DefaultActionsURL = defaultActionsURLGitHub
		} else {
			return fmt.Errorf("unsupported [actions] DEFAULT_ACTIONS_URL: %q", urls)
		}
	}

	// don't support to read configuration from [actions]
	Actions.LogStorage, err = getStorage(rootCfg, "actions_log", "", nil)
	if err != nil {
		return err
	}

	actionsSec, _ := rootCfg.GetSection("actions.artifacts")

	Actions.ArtifactStorage, err = getStorage(rootCfg, "actions_artifacts", "", actionsSec)

	return err
}
