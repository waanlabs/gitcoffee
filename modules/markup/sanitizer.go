// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"io"
	"net/url"
	"regexp"
	"sync"

	"code.gitea.io/gitea/modules/setting"

	"github.com/microcosm-cc/bluemonday"
)

// Sanitizer is a protection wrapper of *bluemonday.Policy which does not allow
// any modification to the underlying policies once it's been created.
type Sanitizer struct {
	defaultPolicy    *bluemonday.Policy
	rendererPolicies map[string]*bluemonday.Policy
	init             sync.Once
}

var (
	sanitizer     = &Sanitizer{}
	allowAllRegex = regexp.MustCompile(".+")
)

// NewSanitizer initializes sanitizer with allowed attributes based on settings.
// Multiple calls to this function will only create one instance of Sanitizer during
// entire application lifecycle.
func NewSanitizer() {
	sanitizer.init.Do(func() {
		InitializeSanitizer()
	})
}

// InitializeSanitizer (re)initializes the current sanitizer to account for changes in settings
func InitializeSanitizer() {
	sanitizer.rendererPolicies = map[string]*bluemonday.Policy{}
	sanitizer.defaultPolicy = createDefaultPolicy()

	for name, renderer := range renderers {
		sanitizerRules := renderer.SanitizerRules()
		if len(sanitizerRules) > 0 {
			policy := createDefaultPolicy()
			addSanitizerRules(policy, sanitizerRules)
			sanitizer.rendererPolicies[name] = policy
		}
	}
}

func createDefaultPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()

	// For JS code copy and Mermaid loading state
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^code-block( is-loading)?$`)).OnElements("pre")

	// For color preview
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^color-preview$`)).OnElements("span")

	// For attention
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^attention-\w+$`)).OnElements("strong")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^attention-icon attention-\w+$`)).OnElements("span", "strong")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^svg octicon-\w+$`)).OnElements("svg")
	policy.AllowAttrs("viewBox", "width", "height", "aria-hidden").OnElements("svg")
	policy.AllowAttrs("fill-rule", "d").OnElements("path")

	// For Chroma markdown plugin
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^(chroma )?language-[\w-]+( display)?( is-loading)?$`)).OnElements("code")

	// Checkboxes
	policy.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	policy.AllowAttrs("checked", "disabled", "data-source-position").OnElements("input")

	// Custom URL-Schemes
	if len(setting.Markdown.CustomURLSchemes) > 0 {
		policy.AllowURLSchemes(setting.Markdown.CustomURLSchemes...)
	} else {
		policy.AllowURLSchemesMatching(allowAllRegex)

		// Even if every scheme is allowed, these three are blocked for security reasons
		disallowScheme := func(*url.URL) bool {
			return false
		}
		policy.AllowURLSchemeWithCustomPolicy("javascript", disallowScheme)
		policy.AllowURLSchemeWithCustomPolicy("vbscript", disallowScheme)
		policy.AllowURLSchemeWithCustomPolicy("data", disallowScheme)
	}

	// Allow classes for anchors
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`ref-issue( ref-external-issue)?`)).OnElements("a")

	// Allow classes for task lists
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`task-list-item`)).OnElements("li")

	// Allow classes for org mode list item status.
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^(unchecked|checked|indeterminate)$`)).OnElements("li")

	// Allow icons
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^icon(\s+[\p{L}\p{N}_-]+)+$`)).OnElements("i")

	// Allow unlabelled labels
	policy.AllowNoAttrs().OnElements("label")

	// Allow classes for emojis
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`emoji`)).OnElements("img")

	// Allow icons, emojis, chroma syntax and keyword markup on span
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^((icon(\s+[\p{L}\p{N}_-]+)+)|(emoji)|(language-math display)|(language-math inline))$|^([a-z][a-z0-9]{0,2})$|^` + keywordClass + `$`)).OnElements("span")

	// Allow 'style' attribute on text elements.
	policy.AllowAttrs("style").OnElements("span", "p")

	// Allow 'color' and 'background-color' properties for the style attribute on text elements.
	policy.AllowStyles("color", "background-color").OnElements("span", "p")

	// Allow generally safe attributes
	generalSafeAttrs := []string{
		"abbr", "accept", "accept-charset",
		"accesskey", "action", "align", "alt",
		"aria-describedby", "aria-hidden", "aria-label", "aria-labelledby",
		"axis", "border", "cellpadding", "cellspacing", "char",
		"charoff", "charset", "checked",
		"clear", "cols", "colspan", "color",
		"compact", "coords", "datetime", "dir",
		"disabled", "enctype", "for", "frame",
		"headers", "height", "hreflang",
		"hspace", "ismap", "label", "lang",
		"maxlength", "media", "method",
		"multiple", "name", "nohref", "noshade",
		"nowrap", "open", "prompt", "readonly", "rel", "rev",
		"rows", "rowspan", "rules", "scope",
		"selected", "shape", "size", "span",
		"start", "summary", "tabindex", "target",
		"title", "type", "usemap", "valign", "value",
		"vspace", "width", "itemprop",
	}

	generalSafeElements := []string{
		"h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8", "br", "b", "i", "strong", "em", "a", "pre", "code", "img", "tt",
		"div", "ins", "del", "sup", "sub", "p", "ol", "ul", "table", "thead", "tbody", "tfoot", "blockquote",
		"dl", "dt", "dd", "kbd", "q", "samp", "var", "hr", "ruby", "rt", "rp", "li", "tr", "td", "th", "s", "strike", "summary",
		"details", "caption", "figure", "figcaption",
		"abbr", "bdo", "cite", "dfn", "mark", "small", "span", "time", "video", "wbr",
	}

	policy.AllowAttrs(generalSafeAttrs...).OnElements(generalSafeElements...)

	policy.AllowAttrs("src", "autoplay", "controls").OnElements("video")

	policy.AllowAttrs("itemscope", "itemtype").OnElements("div")

	// FIXME: Need to handle longdesc in img but there is no easy way to do it

	// Custom keyword markup
	addSanitizerRules(policy, setting.ExternalSanitizerRules)

	return policy
}

func addSanitizerRules(policy *bluemonday.Policy, rules []setting.MarkupSanitizerRule) {
	for _, rule := range rules {
		if rule.AllowDataURIImages {
			policy.AllowDataURIImages()
		}
		if rule.Element != "" {
			if rule.Regexp != nil {
				policy.AllowAttrs(rule.AllowAttr).Matching(rule.Regexp).OnElements(rule.Element)
			} else {
				policy.AllowAttrs(rule.AllowAttr).OnElements(rule.Element)
			}
		}
	}
}

// Sanitize takes a string that contains a HTML fragment or document and applies policy whitelist.
func Sanitize(s string) string {
	NewSanitizer()
	return sanitizer.defaultPolicy.Sanitize(s)
}

// SanitizeReader sanitizes a Reader
func SanitizeReader(r io.Reader, renderer string, w io.Writer) error {
	NewSanitizer()
	policy, exist := sanitizer.rendererPolicies[renderer]
	if !exist {
		policy = sanitizer.defaultPolicy
	}
	return policy.SanitizeReaderToWriter(r, w)
}
