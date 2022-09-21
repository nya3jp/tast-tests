// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package openfds contains support code for the security.OpenFDs test.
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

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Expectation represents expected path and file mode for file descriptors in
// a process.
// PathPattern is the string rather than regexp.Regexp, in order to avoid
// verbose expectation data list in the test.
type Expectation struct {
	// Regex pattern to be matched with the opened file.
	// This is full-string match pattern.
	PathPattern string

	// Compiled regex pattern from |PathPattern|.
	pathRegex *regexp.Regexp

	// List of possible permissions of the symlink under /proc/{PID}/fd/*
	// Note: The "mode" on the link tells us if the file is opened for
	// read/write. We are more interested in that than the permissions of
	// the file on the fs.
	Modes []uint32
}

type fileMode struct {
	path  string // Path of the opened file.
	mode  uint32 // Mode of the opened file.
	lmode uint32 // Mode of the lstat(2) of /proc/*/fd/{FD} file.
}

func (m fileMode) String() string {
	return fmt.Sprintf("{path: %q mode: %o lmode: %o}", m.path, m.mode, m.lmode)
}

// DumpFDs outputs the current file descriptor status into a file at path.
func DumpFDs(ctx context.Context, path string) error {
	// To expand the glob pattern, use shell.
	cmd := testexec.CommandContext(ctx, "sh", "-c", "ls -l /proc/[0-9]*/fd")
	// Intentionally ignore errors. Some files under /proc/*/fd is not
	// accessible by permission.
	o, _ := cmd.CombinedOutput()
	return ioutil.WriteFile(path, o, 0644)
}

// expectType returns whether the given mode is allowed or not for an open
// file of Chrome.
func expectType(mode uint32) bool {
	// This is allowed fd-type check, suitable for Chrome processes.
	// Notably, this omits S_ISDIR.
	switch mode & unix.S_IFMT {
	case unix.S_IFCHR, unix.S_IFSOCK, unix.S_IFIFO, unix.S_IFREG:
		return true
	}

	// Checks if mode represents an "anonymous inode" or not.
	return (mode & 0770000) == 0
}

// findExpectation returns a corresponding entry in the given
// Expectation array which matches to the given path.
func findExpectation(path string, es []Expectation) (*Expectation, error) {
	for _, e := range es {
		if e.pathRegex.MatchString(path) {
			return &e, nil
		}
	}
	return nil, errors.Errorf("no mode expectation found for path: %s", path)
}

// expectMode checks if the given lmode is contained in expectModes.
func expectMode(lmode uint32, expectModes []uint32) bool {
	m := lmode & 0777
	for _, e := range expectModes {
		if m == e {
			return true
		}
	}
	return false
}

// openFileModes extracts all the opened files of the p, with annotating
// mode of the original file and mode of the lstat(2) of the /proc/*/fd/{FD}
// file.
func openFileModes(ctx context.Context, p *process.Process) ([]fileMode, error) {
	// Note: current gopsutil is old so that context.Context is not
	// supported.
	// Note: there's very rare possibility of race condition here, if
	// the target process is kill'ed and collected, then a number of
	// process is created, then PID could be reused.
	// Though, practically it should rarely happen.
	openfiles, err := p.OpenFiles()
	if err != nil {
		return nil, err
	}

	var ret []fileMode
	for _, f := range openfiles {
		// There's a chance that the fd is closed between taking
		// openfiles snapshot and calling stat/lstat below.
		// So, in case of error, just skip errors with logging.
		// Note: there's also race condition here. The FD may be
		// closed and then the FD may be reassigned to newly opened
		// file descriptor in the process. Practically this test is
		// stable enough, so leaving it now.
		fdpath := filepath.Join(
			"/proc", strconv.Itoa(int(p.Pid)), "fd", strconv.Itoa(int(f.Fd)))
		linfo, err := os.Lstat(fdpath)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to lstat %s: %v", fdpath, err)
			continue
		}

		// It is necessary to stat via symlink for, e.g., taking the
		// stat(2) for socket, pipe, anon_inode, or a file which is
		// already deleted.
		info, err := os.Stat(fdpath)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to stat %s: %v", fdpath, err)
			continue
		}

		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, errors.Errorf("failed to obtain stat_t for %s", f.Path)
		}
		ret = append(ret, fileMode{f.Path, st.Mode, uint32(linfo.Mode())})
	}
	return ret, nil
}

// Expect tests the file types and file modes of the opened files for the
// given Chrome process p.
// allowDirs should be true if open directories are allowed.
// es is a list of expected file modes. Please see also the comment of
// Expectation for details.
func Expect(ctx context.Context, s *testing.State, allowDirs bool, p *process.Process, es []Expectation) {
	// Create Regex object if necessary.
	for i := range es {
		if es[i].pathRegex != nil {
			continue
		}
		// PathPattern is full match, so wrap by "^(...)$".
		es[i].pathRegex = regexp.MustCompile("^(" + es[i].PathPattern + ")$")
	}

	files, err := openFileModes(ctx, p)
	if err != nil {
		s.Errorf("Failed to obtain opened fds for %v: %v", p, err)
		return
	}

	// Check for each opened file.
	for _, f := range files {
		// Skip file type check iff allowDirs is true.
		if !allowDirs && !expectType(f.mode) {
			s.Errorf("Unexpected file type %v for %v", f, p)
		}

		if e, err := findExpectation(f.path, es); err != nil {
			// File path (i.e. the result of readlink(2)) must be
			// listed in the expectation allow-list. If not found,
			// it means an unexpected file is opened, so report
			// an error.
			s.Errorf("Expectation not found for %v: %v", p, err)
		} else if submatches := e.pathRegex.FindStringSubmatch(f.path); len(submatches) == 3 {
			// If an expectation was found, and there are exactly 3 submatches,
			// and the submatch at index 2 is the named capture group "pid";
			// only accept the expectation if the PID in the path matches the PID
			// of the process.
			// 3 submatches are needed because 'pathRegex' is of the form
			// ^(pattern)$, and therefore both submatches[0] and submatches[1] include
			// the entire string.
			if captureName := e.pathRegex.SubexpNames()[2]; captureName != "pid" {
				s.Errorf("Unexpected capture group in regexp %v: got %q; want %q", e.PathPattern, captureName, "pid")
				continue
			}

			if n, err := strconv.Atoi(submatches[2]); err != nil {
				s.Errorf("%v is not a PID: %v", submatches[2], err)
			} else if n != int(p.Pid) {
				s.Errorf("PID in path %v does not match expected PID: got %d; want %d", f.path, n, p.Pid)
			}
		} else if !expectMode(f.lmode, e.Modes) {
			s.Errorf("Unexpected mode for process %v, file %q: got %o; want one of %o", p, f.path, f.lmode, e.Modes)
		}
	}
}
