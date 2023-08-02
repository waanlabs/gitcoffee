// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

func (repo *Repository) getTree(id SHA1) (*Tree, error) {
	gogitTree, err := repo.gogitRepo.TreeObject(id)
	if err != nil {
		return nil, err
	}

	tree := NewTree(repo, id)
	tree.gogitTree = gogitTree
	return tree, nil
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	if len(idStr) != SHAFullLength {
		res, _, err := NewCommand(repo.Ctx, "rev-parse", "--verify").AddDynamicArguments(idStr).RunStdString(&RunOpts{Dir: repo.Path})
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res[:len(res)-1]
		}
	}
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	resolvedID := id
	commitObject, err := repo.gogitRepo.CommitObject(id)
	if err == nil {
		id = SHA1(commitObject.TreeHash)
	}
	treeObject, err := repo.getTree(id)
	if err != nil {
		return nil, err
	}
	treeObject.ResolvedID = resolvedID
	return treeObject, nil
}
