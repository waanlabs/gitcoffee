// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import (
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// BlockRenderer represents a renderer for math Blocks
type BlockRenderer struct{}

// NewBlockRenderer creates a new renderer for math Blocks
func NewBlockRenderer() renderer.NodeRenderer {
	return &BlockRenderer{}
}

// RegisterFuncs registers the renderer for math Blocks
func (r *BlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindBlock, r.renderBlock)
}

func (r *BlockRenderer) writeLines(w util.BufWriter, source []byte, n gast.Node) {
	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		_, _ = w.Write(util.EscapeHTML(line.Value(source)))
	}
}

func (r *BlockRenderer) renderBlock(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*Block)
	if entering {
		_, _ = w.WriteString(`<pre class="code-block is-loading"><code class="chroma language-math display">`)
		r.writeLines(w, source, n)
	} else {
		_, _ = w.WriteString(`</code></pre>` + "\n")
	}
	return gast.WalkContinue, nil
}
