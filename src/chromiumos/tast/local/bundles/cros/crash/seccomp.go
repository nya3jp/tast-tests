// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Seccomp,
		Desc: "Test to check that seccomp policy files are captured",
		Contacts: []string{
			"mutexlox@chromium.org",
			"allenwebb@chromium.org",
			"jorgelo@google.com",
			"cros-telemetry@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

// mustContain returns an error unless the file at the specified path contains the specified line.
func mustContain(filename, line string) error {
	f, err := os.Open(filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open %q", filename)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if s.Text() == line {
			return nil
		}
	}
	if err = s.Err(); err != nil {
		return errors.Wrapf(err, "failed to read %q", filename)
	}
	return errors.Errorf("%q does not contain %q", filename, line)
}

// readPidFromFile recovers the process id passed over the specified file.
func readPidFromFile(f *os.File) (int, error) {
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return -1, errors.New("scan failed")
	}
	return strconv.Atoi(s.Text())
}

// Seccomp verifies that a crash report caused by a seccomp violation will contain the policy path.
func Seccomp(ctx context.Context, s *testing.State) {
	const executable = "/usr/local/libexec/tast/helpers/local/cros/crash.Seccomp.brk"
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(ctx); err != nil {
			s.Error("Failed to tear down crash test: ", err)
		}
	}()

	pidFile, err := ioutil.TempFile("", "crash.Seccomp")
	if err != nil {
		s.Fatal("Failed to get pid file: ", err)
	}
	defer pidFile.Close()

	// Force a seccomp failure with a recoverable pid.
	cmd := testexec.CommandContext(
		ctx,
		"/sbin/minijail0",
		// Write the pid of the sandboxed process to pidFile.
		"-f", pidFile.Name(),
		// Use /dev/null as an empty seccomp policy (i.e. deny all system calls).
		"-S", "/dev/null",
		// Everything after this is the sandboxed command.
		"--",
		executable)
	if err := cmd.Run(); err == nil {
		s.Fatal("Expected crash, but command exited normally")
	}

	// Find the crash files.
	pid, err := readPidFromFile(pidFile)
	if err != nil {
		s.Fatal("Failed to read pid: ", err)
	}

	pattern := fmt.Sprintf("crash_Seccomp_brk.*.%d.*", pid)
	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	// We might not be logged in, so also allow system crash dir.
	crashDirs = append(crashDirs, crash.SystemCrashDir)
	files, err := crash.WaitForCrashFiles(ctx, crashDirs, []string{pattern})
	if err != nil {
		s.Fatal("Failed to wait for crash files: ", err)
	}

	// Check proclog for the expected environment variable and value.
	found := false
	for _, match := range files[pattern] {
		if strings.HasSuffix(match, ".proclog") {
			found = true
			if err = mustContain(match, "SECCOMP_POLICY_PATH=/dev/null"); err != nil {
				s.Error("Failed to find expected string: ", err)
				crash.MoveFilesToOut(ctx, s.OutDir(), match)
			}
		} else if strings.HasSuffix(match, ".meta") {
			contents, err := ioutil.ReadFile(match)
			if err != nil {
				s.Errorf("Couldn't read meta file %s contents: %v", match, err)
				continue
			}
			if !strings.Contains(string(contents), "upload_var_seccomp_blocked_syscall_nr=") {
				s.Error("Failed to find expected seccomp_blocked_syscall_nr")
				crash.MoveFilesToOut(ctx, s.OutDir(), match)
			}
			if !strings.Contains(string(contents), "upload_var_seccomp_proc_pid_syscall=") {
				s.Error("Failed to find expected seccomp_proc_pid_syscall")
				crash.MoveFilesToOut(ctx, s.OutDir(), match)
			}
		}
	}
	if !found {
		s.Error("Failed to find proclog")
	}
	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
