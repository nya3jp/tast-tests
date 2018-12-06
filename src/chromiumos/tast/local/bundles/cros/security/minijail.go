// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Minijail,
		Desc: "Verifies minijail0's basic functionality",
		Attr: []string{"informational"},
	})
}

func Minijail(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"
		exeDir       = "/usr/local/libexec/security_tests/security.Minijail"
		testGlob     = "/usr/local/share/security_tests/security.Minijail/test-*"
	)

	// Test cases consist of test-* shell scripts containing options specified on
	// lines of the form "# <name>: value". This function builds a map of all lines matching
	// this format. Individual options are described below in runTest.
	testOptionRegexp := regexp.MustCompile("^# ([_a-z0-9]+): (.*)")
	getTestOptions := func(p string) map[string]string {
		f, err := os.Open(p)
		if err != nil {
			s.Fatal("Failed to open test file: ", err)
		}
		defer f.Close()

		opts := make(map[string]string)
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			matches := testOptionRegexp.FindStringSubmatch(sc.Text())
			if matches != nil {
				opts[matches[1]] = matches[2]
			}
		}
		if sc.Err() != nil {
			s.Fatalf("Failed to read test file %v: %v", p, sc.Err())
		}
		return opts
	}

	// Commands and arguments within test cases are specified as shell-quoted strings,
	// so they need to be parsed by sh.
	shellCmd := func(cmd string) *testexec.Cmd { return testexec.CommandContext(ctx, "sh", "-c", cmd) }

	runTest := func(testPath string, static bool) {
		// Construct a human-readable test name.
		name := filepath.Base(testPath)
		if static {
			name += ".static"
		} else {
			name += ".non-static"
		}

		// Create a temp dir that the test's setup function (if any) can write to.
		td, err := ioutil.TempDir("", "tast.security.Minijail."+name+".")
		if err != nil {
			s.Fatal("Failed to create temp dir: ", err)
		}
		defer os.RemoveAll(td)

		// Tests can specify a "setup" option containing a shell-quoted command to run before the test.
		// "%T" is replaced by the temp dir's path.
		opts := getTestOptions(testPath)
		if setup, ok := opts["setup"]; ok {
			setup = strings.Replace(setup, "%T", td, -1)
			s.Logf("Running %v setup: %v", name, setup)
			cmd := shellCmd(setup)
			if err := cmd.Run(); err != nil {
				s.Errorf("Failed %v setup: %v", name, err)
				cmd.DumpLog(ctx)
				return
			}
		}

		// Tests can specify an "args" option containing a shell-quoted string that should
		// be appended to the command line immediately after the minijail0 command.
		// "%T" is replaced by the temp dir's path.
		args := opts["args"]
		if _, err := os.Stat("/lib64"); err == nil {
			// Some tests define an "args64" option that references /lib64.
			if args64, ok := opts["args64"]; ok {
				args += " " + args64
			}
		}
		args = strings.Replace(args, "%T", td, -1)

		// Tests can specify an "expected_ugid" option consisting of a space-separated UID and GID.
		// In this case, a writable path is passed to the test, and the test is expected to create a file there.
		// The file's UID and GID are later verified to match the expected values.
		expUID, expGID := -1, -1
		usernsFile := ""
		if expUGID, ok := opts["expected_ugid"]; ok {
			if parts := strings.Fields(expUGID); len(parts) != 2 {
				s.Fatalf("%v has bad expected UID/GID pair %q", testPath, expUGID)
			} else if expUID, err = strconv.Atoi(parts[0]); err != nil {
				s.Fatalf("%v has bad UID %q", testPath, parts[0])
			} else if expGID, err = strconv.Atoi(parts[1]); err != nil {
				s.Fatalf("%v has bad GID %q", testPath, parts[1])
			}

			// Create a separate temp dir explicitly under /tmp for the test to write to.
			// We need to make sure that this path is accessible to non-root users.
			usernsDir, err := ioutil.TempDir("/tmp", "tast.security.Minijail."+name+".")
			if err != nil {
				s.Fatal("Failed to create temp dir: ", err)
			}
			defer os.RemoveAll(usernsDir)
			if err := os.Chmod(usernsDir, 0777); err != nil {
				s.Fatalf("Failed to chmod %v: %v", usernsDir, err)
			}
			usernsFile = filepath.Join(usernsDir, "userns")
		}

		shell := "/bin/bash"
		if static {
			shell = filepath.Join(exeDir, "staticbashexec")
		}

		cmdLine := strings.Join([]string{minijailPath, args, shell, testPath, usernsFile}, " ")
		s.Logf("Running %v command: %v", name, cmdLine)
		cmd := shellCmd(cmdLine)
		if err := cmd.Run(); err != nil {
			s.Errorf("%v failed: %v", name, err)
			cmd.DumpLog(ctx)
		} else if expUID >= 0 || expGID >= 0 {
			fi, err := os.Stat(usernsFile)
			if err != nil {
				s.Errorf("Failed to stat %v userns file: %v", name, err)
			} else if st := fi.Sys().(*syscall.Stat_t); int(st.Uid) != expUID || int(st.Gid) != expGID {
				s.Errorf("%v userns file has UID %v (want %v) and GID %v (want %v)", name, st.Uid, expUID, st.Gid, expGID)
			}
		}
	}

	paths, err := filepath.Glob(testGlob)
	if err != nil {
		s.Fatalf("Failed to look for test files in %v: %v", testGlob, err)
	} else if len(paths) == 0 {
		s.Fatal("No test files found in ", testGlob)
	}
	for _, p := range paths {
		runTest(p, false) // non-static
		runTest(p, true)  // static
	}
}
