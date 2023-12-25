// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// IsObjectExist returns true if given reference exists in the repository.
func (repo *Repository) IsObjectExist(name string) bool {
	if name == "" {
		return false
	}

	_, err := repo.gogitRepo.ResolveRevision(plumbing.Revision(name))

	return err == nil
}

// IsReferenceExist returns true if given reference exists in the repository.
func (repo *Repository) IsReferenceExist(name string) bool {
	if name == "" {
		return false
	}

	reference, err := repo.gogitRepo.Reference(plumbing.ReferenceName(name), true)
	if err != nil {
		return false
	}
	return reference.Type() != plumbing.InvalidReference
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if name == "" {
		return false
	}
	reference, err := repo.gogitRepo.Reference(plumbing.ReferenceName(BranchPrefix+name), true)
	if err != nil {
		return false
	}
	return reference.Type() != plumbing.InvalidReference
}

// GetBranches returns branches from the repository, skipping "skip" initial branches and
// returning at most "limit" branches, or all branches if "limit" is 0.
// Branches are returned with sort of `-commiterdate` as the nogogit
// implementation. This requires full fetch, sort and then the
// skip/limit applies later as gogit returns in undefined order.
func (repo *Repository) GetBranchNames(skip, limit int) ([]string, int, error) {
	type BranchData struct {
		name          string
		committerDate int64
	}
	var branchData []BranchData

	branchIter, err := repo.gogitRepo.Branches()
	if err != nil {
		return nil, 0, err
	}

	_ = branchIter.ForEach(func(branch *plumbing.Reference) error {
		obj, err := repo.gogitRepo.CommitObject(branch.Hash())
		if err != nil {
			// skip branch if can't find commit
			return nil
		}

		branchData = append(branchData, BranchData{strings.TrimPrefix(branch.Name().String(), BranchPrefix), obj.Committer.When.Unix()})
		return nil
	})

	sort.Slice(branchData, func(i, j int) bool {
		return !(branchData[i].committerDate < branchData[j].committerDate)
	})

	var branchNames []string
	maxPos := len(branchData)
	if limit > 0 {
		maxPos = min(skip+limit, maxPos)
	}
	for i := skip; i < maxPos; i++ {
		branchNames = append(branchNames, branchData[i].name)
	}

	return branchNames, len(branchData), nil
}

// WalkReferences walks all the references from the repository
// refType should be empty, ObjectTag or ObjectBranch. All other values are equivalent to empty.
func WalkReferences(ctx context.Context, repoPath string, walkfn func(sha1, refname string) error) (int, error) {
	repo := RepositoryFromContext(ctx, repoPath)
	if repo == nil {
		var err error
		repo, err = OpenRepository(ctx, repoPath)
		if err != nil {
			return 0, err
		}
		defer repo.Close()
	}

	i := 0
	iter, err := repo.gogitRepo.References()
	if err != nil {
		return i, err
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		err := walkfn(ref.Hash().String(), string(ref.Name()))
		i++
		return err
	})
	return i, err
}

// WalkReferences walks all the references from the repository
func (repo *Repository) WalkReferences(arg ObjectType, skip, limit int, walkfn func(sha1, refname string) error) (int, error) {
	i := 0
	var iter storer.ReferenceIter
	var err error
	switch arg {
	case ObjectTag:
		iter, err = repo.gogitRepo.Tags()
	case ObjectBranch:
		iter, err = repo.gogitRepo.Branches()
	default:
		iter, err = repo.gogitRepo.References()
	}
	if err != nil {
		return i, err
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if i < skip {
			i++
			return nil
		}
		err := walkfn(ref.Hash().String(), string(ref.Name()))
		i++
		if err != nil {
			return err
		}
		if limit != 0 && i >= skip+limit {
			return storer.ErrStop
		}
		return nil
	})
	return i, err
}

// GetRefsBySha returns all references filtered with prefix that belong to a sha commit hash
func (repo *Repository) GetRefsBySha(sha, prefix string) ([]string, error) {
	var revList []string
	iter, err := repo.gogitRepo.References()
	if err != nil {
		return nil, err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Hash().String() == sha && strings.HasPrefix(string(ref.Name()), prefix) {
			revList = append(revList, string(ref.Name()))
		}
		return nil
	})
	return revList, err
}
