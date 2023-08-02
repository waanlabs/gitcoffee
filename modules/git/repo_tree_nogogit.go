// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"io"
)

func (repo *Repository) getTree(id SHA1) (*Tree, error) {
	wr, rd, cancel := repo.CatFileBatch(repo.Ctx)
	defer cancel()

	_, _ = wr.Write([]byte(id.String() + "\n"))

	// ignore the SHA
	_, typ, size, err := ReadBatchLine(rd)
	if err != nil {
		return nil, err
	}

	switch typ {
	case "tag":
		resolvedID := id
		data, err := io.ReadAll(io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(data)
		if err != nil {
			return nil, err
		}
		commit, err := tag.Commit(repo)
		if err != nil {
			return nil, err
		}
		commit.Tree.ResolvedID = resolvedID
		return &commit.Tree, nil
	case "commit":
		commit, err := CommitFromReader(repo, id, io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		if _, err := rd.Discard(1); err != nil {
			return nil, err
		}
		commit.Tree.ResolvedID = commit.ID
		return &commit.Tree, nil
	case "tree":
		tree := NewTree(repo, id)
		tree.ResolvedID = id
		tree.entries, err = catBatchParseTreeEntries(tree, rd, size)
		if err != nil {
			return nil, err
		}
		tree.entriesParsed = true
		return tree, nil
	default:
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	if len(idStr) != SHAFullLength {
		res, err := repo.GetRefCommitID(idStr)
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res
		}
	}
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}

	return repo.getTree(id)
}
