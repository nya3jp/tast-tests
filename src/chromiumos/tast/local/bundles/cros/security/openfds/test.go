// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package openfds

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Expectation of each file descriptor's mode.
type Expectation struct {
	// Regex pattern to be matched with the opened file.
	PathPattern string

	// List of possible permissions.
	Modes []uint32
}

// Internal representation of the expectation.
type compiledExpectation struct {
	pathRegex *regexp.Regexp
	modes     []uint32
}

type fileMode struct {
	path  string // Path of the opened file.
	mode  uint32 // Mode of the opened file.
	lmode uint32 // Mode of the lstat(2) of /proc/*/fd/{FD} file.
}

func (e *compiledExpectation) expect(f *fileMode) bool {
	// Make sure if this expectation is for the given path.
	if !e.pathRegex.MatchString(f.path) {
		return false
	}

	// Make sure if the file mode is one of the expected one.
	m := f.lmode & 0777
	for _, em := range e.modes {
		if m == em {
			return true
		}
	}
	return false
}

// Dumps the current file descriptor status into a file at |path|.
func DumpFds(ctx context.Context, path string) error {
	cmd := testexec.CommandContext(ctx, "dash", "-c", "ls -l /proc/*[0-9]*/fd")
	// Intentionally ignore errors. Some files under /proc/*/fd is not
	// accessible by permission.
	o, _ := cmd.CombinedOutput()
	return ioutil.WriteFile(path, o, 0644)
}

// Whitelist fd-type check, suitable for Chrome processes. Notably,
// this omits S_ISDIR.
func expectType(mode uint32) bool {
	switch mode & syscall.S_IFMT {
	case syscall.S_IFCHR, syscall.S_IFSOCK, syscall.S_IFIFO, syscall.S_IFREG:
		return true
	}

	return (mode & 0770000) == 0
}

// Checks if the file mode of the given file is one of the expected one.
// Returns true on success.
func expectMode(f *fileMode, es []compiledExpectation) bool {
	for _, e := range es {
		if e.expect(f) {
			return true
		}
	}
	return false
}

// Extracts all the opened files of the |p|, with annotating mode of the
// original file and mode of the lstat(2) of the /proc/*/fd/{FD} file.
func openFileModes(ctx context.Context, p *process.Process) ([]fileMode, error) {
	// Note: current gopsutil is old so that context.Context is not
	// supported.
	openfiles, err := p.OpenFiles()
	if err != nil {
		return nil, err
	}

	ret := make([]fileMode, 0)
	for _, f := range openfiles {
		// There's a chance that the fd is closed between taking
		// openfiles snapshot and calling stat/lstat below.
		// So, in case of error, just skip errors with logging.
		fdpath := filepath.Join(
			"/proc", strconv.Itoa(int(p.Pid)), "fd", strconv.Itoa(int(f.Fd)))
		linfo, err := os.Lstat(fdpath)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to lstat %s: %v", fdpath, err)
			continue
		}
		info, err := os.Stat(f.Path)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to stat %s: %v", f.Path, err)
			continue
		}

		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fmt.Errorf("Failed to obtain stat_t for %s", f.Path)
		}
		ret = append(ret, fileMode{f.Path, st.Mode, uint32(linfo.Mode())})
	}
	return ret, nil
}

func compile(es []Expectation) []compiledExpectation {
	ret := make([]compiledExpectation, len(es))
	for i, e := range es {
		ret[i] = compiledExpectation{
			regexp.MustCompile(e.PathPattern),
			e.Modes,
		}
	}
	return ret
}

func Expect(s *testing.State, ctx context.Context, asan bool, p *process.Process, e []Expectation) {
	cs := compile(e)
	files, err := openFileModes(ctx, p)
	if err != nil {
		s.Errorf("Failed to obtain opened fds for %v: %v", p, err)
		return
	}

	if !asan {
		for _, f := range files {
			if !expectType(f.mode) {
				s.Errorf("Unexpected file type: %v", f)
			}
		}
	}

	for _, f := range files {
		if !expectMode(&f, cs) {
			s.Errorf("Unexpected file mode at %v", f)
		}
	}
}
