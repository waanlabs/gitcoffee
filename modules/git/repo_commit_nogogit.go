// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
	"errors"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// ResolveReference resolves a name to a reference
func (repo *Repository) ResolveReference(name string) (string, error) {
	stdout, _, err := NewCommand(repo.Ctx, "show-ref", "--hash").AddDynamicArguments(name).RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return "", ErrNotExist{name, ""}
	}

	return stdout, nil
}

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	wr, rd, cancel := repo.CatFileBatchCheck(repo.Ctx)
	defer cancel()
	_, err := wr.Write([]byte(name + "\n"))
	if err != nil {
		return "", err
	}
	shaBs, _, _, err := ReadBatchLine(rd)
	if IsErrNotExist(err) {
		return "", ErrNotExist{name, ""}
	}

	return string(shaBs), nil
}

// SetReference sets the commit ID string of given reference (e.g. branch or tag).
func (repo *Repository) SetReference(name, commitID string) error {
	_, _, err := NewCommand(repo.Ctx, "update-ref").AddDynamicArguments(name, commitID).RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// RemoveReference removes the given reference (e.g. branch or tag).
func (repo *Repository) RemoveReference(name string) error {
	_, _, err := NewCommand(repo.Ctx, "update-ref", "--no-deref", "-d").AddDynamicArguments(name).RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	_, _, err := NewCommand(repo.Ctx, "cat-file", "-e").AddDynamicArguments(name).RunStdString(&RunOpts{Dir: repo.Path})
	return err == nil
}

func (repo *Repository) getCommit(id SHA1) (*Commit, error) {
	wr, rd, cancel := repo.CatFileBatch(repo.Ctx)
	defer cancel()

	_, _ = wr.Write([]byte(id.String() + "\n"))

	return repo.getCommitFromBatchReader(rd, id)
}

func (repo *Repository) getCommitFromBatchReader(rd *bufio.Reader, id SHA1) (*Commit, error) {
	_, typ, size, err := ReadBatchLine(rd)
	if err != nil {
		if errors.Is(err, io.EOF) || IsErrNotExist(err) {
			return nil, ErrNotExist{ID: id.String()}
		}
		return nil, err
	}

	switch typ {
	case "missing":
		return nil, ErrNotExist{ID: id.String()}
	case "tag":
		// then we need to parse the tag
		// and load the commit
		data, err := io.ReadAll(io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
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

		return commit, nil
	case "commit":
		commit, err := CommitFromReader(repo, id, io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}

		return commit, nil
	default:
		log.Debug("Unknown typ: %s", typ)
		_, err = rd.Discard(int(size) + 1)
		if err != nil {
			return nil, err
		}
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// ConvertToSHA1 returns a Hash object from a potential ID string
func (repo *Repository) ConvertToSHA1(commitID string) (SHA1, error) {
	if len(commitID) == SHAFullLength && IsValidSHAPattern(commitID) {
		sha1, err := NewIDFromString(commitID)
		if err == nil {
			return sha1, nil
		}
	}

	wr, rd, cancel := repo.CatFileBatchCheck(repo.Ctx)
	defer cancel()
	_, err := wr.Write([]byte(commitID + "\n"))
	if err != nil {
		return SHA1{}, err
	}
	sha, _, _, err := ReadBatchLine(rd)
	if err != nil {
		if IsErrNotExist(err) {
			return SHA1{}, ErrNotExist{commitID, ""}
		}
		return SHA1{}, err
	}

	return MustIDFromString(string(sha)), nil
}
