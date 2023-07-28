// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"runtime"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
)

// WriteCloserError wraps an io.WriteCloser with an additional CloseWithError function
type WriteCloserError interface {
	io.WriteCloser
	CloseWithError(err error) error
}

// EnsureValidGitRepository runs git rev-parse in the repository path - thus ensuring that the repository is a valid repository.
// Run before opening git cat-file.
// This is needed otherwise the git cat-file will hang for invalid repositories.
func EnsureValidGitRepository(ctx context.Context, repoPath string) error {
	stderr := strings.Builder{}
	err := NewCommand(ctx, "rev-parse").
		SetDescription(fmt.Sprintf("%s rev-parse [repo_path: %s]", GitExecutable, repoPath)).
		Run(&RunOpts{
			Dir:    repoPath,
			Stderr: &stderr,
		})
	if err != nil {
		return ConcatenateError(err, (&stderr).String())
	}
	return nil
}

// CatFileBatchCheck opens git cat-file --batch-check in the provided repo and returns a stdin pipe, a stdout reader and cancel function
func CatFileBatchCheck(ctx context.Context, repoPath string) (WriteCloserError, *bufio.Reader, func()) {
	batchStdinReader, batchStdinWriter := io.Pipe()
	batchStdoutReader, batchStdoutWriter := io.Pipe()
	ctx, ctxCancel := context.WithCancel(ctx)
	closed := make(chan struct{})
	cancel := func() {
		ctxCancel()
		_ = batchStdoutReader.Close()
		_ = batchStdinWriter.Close()
		<-closed
	}

	// Ensure cancel is called as soon as the provided context is cancelled
	go func() {
		<-ctx.Done()
		cancel()
	}()

	_, filename, line, _ := runtime.Caller(2)
	filename = strings.TrimPrefix(filename, callerPrefix)

	go func() {
		stderr := strings.Builder{}
		err := NewCommand(ctx, "cat-file", "--batch-check").
			SetDescription(fmt.Sprintf("%s cat-file --batch-check [repo_path: %s] (%s:%d)", GitExecutable, repoPath, filename, line)).
			Run(&RunOpts{
				Dir:    repoPath,
				Stdin:  batchStdinReader,
				Stdout: batchStdoutWriter,
				Stderr: &stderr,
			})
		if err != nil {
			_ = batchStdoutWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
			_ = batchStdinReader.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = batchStdoutWriter.Close()
			_ = batchStdinReader.Close()
		}
		close(closed)
	}()

	// For simplicities sake we'll use a buffered reader to read from the cat-file --batch-check
	batchReader := bufio.NewReader(batchStdoutReader)

	return batchStdinWriter, batchReader, cancel
}

// CatFileBatch opens git cat-file --batch in the provided repo and returns a stdin pipe, a stdout reader and cancel function
func CatFileBatch(ctx context.Context, repoPath string) (WriteCloserError, *bufio.Reader, func()) {
	// We often want to feed the commits in order into cat-file --batch, followed by their trees and sub trees as necessary.
	// so let's create a batch stdin and stdout
	batchStdinReader, batchStdinWriter := io.Pipe()
	batchStdoutReader, batchStdoutWriter := nio.Pipe(buffer.New(32 * 1024))
	ctx, ctxCancel := context.WithCancel(ctx)
	closed := make(chan struct{})
	cancel := func() {
		ctxCancel()
		_ = batchStdinWriter.Close()
		_ = batchStdoutReader.Close()
		<-closed
	}

	// Ensure cancel is called as soon as the provided context is cancelled
	go func() {
		<-ctx.Done()
		cancel()
	}()

	_, filename, line, _ := runtime.Caller(2)
	filename = strings.TrimPrefix(filename, callerPrefix)

	go func() {
		stderr := strings.Builder{}
		err := NewCommand(ctx, "cat-file", "--batch").
			SetDescription(fmt.Sprintf("%s cat-file --batch [repo_path: %s] (%s:%d)", GitExecutable, repoPath, filename, line)).
			Run(&RunOpts{
				Dir:    repoPath,
				Stdin:  batchStdinReader,
				Stdout: batchStdoutWriter,
				Stderr: &stderr,
			})
		if err != nil {
			_ = batchStdoutWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
			_ = batchStdinReader.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = batchStdoutWriter.Close()
			_ = batchStdinReader.Close()
		}
		close(closed)
	}()

	// For simplicities sake we'll us a buffered reader to read from the cat-file --batch
	batchReader := bufio.NewReaderSize(batchStdoutReader, 32*1024)

	return batchStdinWriter, batchReader, cancel
}

// ReadBatchLine reads the header line from cat-file --batch
// We expect:
// <sha> SP <type> SP <size> LF
// sha is a 40byte not 20byte here
func ReadBatchLine(rd *bufio.Reader) (sha []byte, typ string, size int64, err error) {
	typ, err = rd.ReadString('\n')
	if err != nil {
		return
	}
	if len(typ) == 1 {
		typ, err = rd.ReadString('\n')
		if err != nil {
			return
		}
	}
	idx := strings.IndexByte(typ, ' ')
	if idx < 0 {
		log.Debug("missing space typ: %s", typ)
		err = ErrNotExist{ID: string(sha)}
		return
	}
	sha = []byte(typ[:idx])
	typ = typ[idx+1:]

	idx = strings.IndexByte(typ, ' ')
	if idx < 0 {
		err = ErrNotExist{ID: string(sha)}
		return
	}

	sizeStr := typ[idx+1 : len(typ)-1]
	typ = typ[:idx]

	size, err = strconv.ParseInt(sizeStr, 10, 64)
	return sha, typ, size, err
}

// ReadTagObjectID reads a tag object ID hash from a cat-file --batch stream, throwing away the rest of the stream.
func ReadTagObjectID(rd *bufio.Reader, size int64) (string, error) {
	var id string
	var n int64
headerLoop:
	for {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		n += int64(len(line))
		idx := bytes.Index(line, []byte{' '})
		if idx < 0 {
			continue
		}

		if string(line[:idx]) == "object" {
			id = string(line[idx+1 : len(line)-1])
			break headerLoop
		}
	}

	// Discard the rest of the tag
	discard := size - n + 1
	for discard > math.MaxInt32 {
		_, err := rd.Discard(math.MaxInt32)
		if err != nil {
			return id, err
		}
		discard -= math.MaxInt32
	}
	_, err := rd.Discard(int(discard))
	return id, err
}

// ReadTreeID reads a tree ID from a cat-file --batch stream, throwing away the rest of the stream.
func ReadTreeID(rd *bufio.Reader, size int64) (string, error) {
	var id string
	var n int64
headerLoop:
	for {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		n += int64(len(line))
		idx := bytes.Index(line, []byte{' '})
		if idx < 0 {
			continue
		}

		if string(line[:idx]) == "tree" {
			id = string(line[idx+1 : len(line)-1])
			break headerLoop
		}
	}

	// Discard the rest of the commit
	discard := size - n + 1
	for discard > math.MaxInt32 {
		_, err := rd.Discard(math.MaxInt32)
		if err != nil {
			return id, err
		}
		discard -= math.MaxInt32
	}
	_, err := rd.Discard(int(discard))
	return id, err
}

// git tree files are a list:
// <mode-in-ascii> SP <fname> NUL <20-byte SHA>
//
// Unfortunately this 20-byte notation is somewhat in conflict to all other git tools
// Therefore we need some method to convert these 20-byte SHAs to a 40-byte SHA

// constant hextable to help quickly convert between 20byte and 40byte hashes
const hextable = "0123456789abcdef"

// To40ByteSHA converts a 20-byte SHA into a 40-byte sha. Input and output can be the
// same 40 byte slice to support in place conversion without allocations.
// This is at least 100x quicker that hex.EncodeToString
// NB This requires that out is a 40-byte slice
func To40ByteSHA(sha, out []byte) []byte {
	for i := 19; i >= 0; i-- {
		v := sha[i]
		vhi, vlo := v>>4, v&0x0f
		shi, slo := hextable[vhi], hextable[vlo]
		out[i*2], out[i*2+1] = shi, slo
	}
	return out
}

// ParseTreeLine reads an entry from a tree in a cat-file --batch stream
// This carefully avoids allocations - except where fnameBuf is too small.
// It is recommended therefore to pass in an fnameBuf large enough to avoid almost all allocations
//
// Each line is composed of:
// <mode-in-ascii-dropping-initial-zeros> SP <fname> NUL <20-byte SHA>
//
// We don't attempt to convert the 20-byte SHA to 40-byte SHA to save a lot of time
func ParseTreeLine(rd *bufio.Reader, modeBuf, fnameBuf, shaBuf []byte) (mode, fname, sha []byte, n int, err error) {
	var readBytes []byte

	// Read the Mode & fname
	readBytes, err = rd.ReadSlice('\x00')
	if err != nil {
		return
	}
	idx := bytes.IndexByte(readBytes, ' ')
	if idx < 0 {
		log.Debug("missing space in readBytes ParseTreeLine: %s", readBytes)

		err = &ErrNotExist{}
		return
	}

	n += idx + 1
	copy(modeBuf, readBytes[:idx])
	if len(modeBuf) >= idx {
		modeBuf = modeBuf[:idx]
	} else {
		modeBuf = append(modeBuf, readBytes[len(modeBuf):idx]...)
	}
	mode = modeBuf

	readBytes = readBytes[idx+1:]

	// Deal with the fname
	copy(fnameBuf, readBytes)
	if len(fnameBuf) > len(readBytes) {
		fnameBuf = fnameBuf[:len(readBytes)]
	} else {
		fnameBuf = append(fnameBuf, readBytes[len(fnameBuf):]...)
	}
	for err == bufio.ErrBufferFull {
		readBytes, err = rd.ReadSlice('\x00')
		fnameBuf = append(fnameBuf, readBytes...)
	}
	n += len(fnameBuf)
	if err != nil {
		return
	}
	fnameBuf = fnameBuf[:len(fnameBuf)-1]
	fname = fnameBuf

	// Deal with the 20-byte SHA
	idx = 0
	for idx < 20 {
		var read int
		read, err = rd.Read(shaBuf[idx:20])
		n += read
		if err != nil {
			return
		}
		idx += read
	}
	sha = shaBuf
	return mode, fname, sha, n, err
}

var callerPrefix string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	callerPrefix = strings.TrimSuffix(filename, "modules/git/batch_reader.go")
}
