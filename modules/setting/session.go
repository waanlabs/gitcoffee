// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

// SessionConfig defines Session settings
var SessionConfig = struct {
	OriginalProvider string
	Provider         string
	// Provider configuration, it's corresponding to provider.
	ProviderConfig string
	// Cookie name to save session ID. Default is "MacaronSession".
	CookieName string
	// Cookie path to store. Default is "/". HINT: there was a bug, the old value doesn't have trailing slash, and could be empty "".
	CookiePath string
	// GC interval time in seconds. Default is 3600.
	Gclifetime int64
	// Max life time in seconds. Default is whatever GC interval time is.
	Maxlifetime int64
	// Use HTTPS only. Default is false.
	Secure bool
	// Cookie domain name. Default is empty.
	Domain string
	// SameSite declares if your cookie should be restricted to a first-party or same-site context. Valid strings are "none", "lax", "strict". Default is "lax"
	SameSite http.SameSite
}{
	CookieName:  "i_like_gitea",
	Gclifetime:  86400,
	Maxlifetime: 86400,
	SameSite:    http.SameSiteLaxMode,
}

func loadSessionFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("session")
	SessionConfig.Provider = sec.Key("PROVIDER").In("memory",
		[]string{"memory", "file", "redis", "mysql", "postgres", "couchbase", "memcache", "db"})
	SessionConfig.ProviderConfig = strings.Trim(sec.Key("PROVIDER_CONFIG").MustString(path.Join(AppDataPath, "sessions")), "\" ")
	if SessionConfig.Provider == "file" && !filepath.IsAbs(SessionConfig.ProviderConfig) {
		SessionConfig.ProviderConfig = path.Join(AppWorkPath, SessionConfig.ProviderConfig)
	}
	SessionConfig.CookieName = sec.Key("COOKIE_NAME").MustString("i_like_gitea")
	SessionConfig.CookiePath = AppSubURL + "/" // there was a bug, old code only set CookePath=AppSubURL, no trailing slash
	SessionConfig.Secure = sec.Key("COOKIE_SECURE").MustBool(false)
	SessionConfig.Gclifetime = sec.Key("GC_INTERVAL_TIME").MustInt64(86400)
	SessionConfig.Maxlifetime = sec.Key("SESSION_LIFE_TIME").MustInt64(86400)
	SessionConfig.Domain = sec.Key("DOMAIN").String()
	samesiteString := sec.Key("SAME_SITE").In("lax", []string{"none", "lax", "strict"})
	switch strings.ToLower(samesiteString) {
	case "none":
		SessionConfig.SameSite = http.SameSiteNoneMode
	case "strict":
		SessionConfig.SameSite = http.SameSiteStrictMode
	default:
		SessionConfig.SameSite = http.SameSiteLaxMode
	}
	shadowConfig, err := json.Marshal(SessionConfig)
	if err != nil {
		log.Fatal("Can't shadow session config: %v", err)
	}
	SessionConfig.ProviderConfig = string(shadowConfig)
	SessionConfig.OriginalProvider = SessionConfig.Provider
	SessionConfig.Provider = "VirtualSession"

	log.Info("Session Service Enabled")
}
