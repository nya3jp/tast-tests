// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShellNoexec,
		Desc: "Checks that shell's noexec mode is active",
		Contacts: []string{
			"vapier@chromium.org", // Author
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func ShellNoexec(ctx context.Context, s *testing.State) {
	// This test causes intentional crashes. Clean up after it.
	defer func() {
		crashes, err := crash.GetCrashes(crash.DefaultDirs()...)
		if err != nil {
			s.Error("Failed to get crash files: ", err)
			return
		}
		for _, exec := range []string{"dash", "bash", "sh"} {
			s.Log("Deleting (expected) crash file(s) for ", exec)
			for _, p := range crashes {
				if fn := filepath.Base(p); !strings.HasPrefix(fn, exec+".") {
					continue
				}
				if err := os.Remove(p); err != nil {
					s.Errorf("Failed to delete %v: %v", p, err)
				}
			}
		}
	}()

	runShell := func(shell string, should_fail, exec_path bool, path string) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		var err error

		// Create the test script under the path.
		script := path + "/.noexec-test.sh"
		var dst io.WriteCloser
		if dst, err = os.Create(script); err != nil {
			s.Fatalf("Failed to create %v: %v", script, err)
		}
		defer os.Remove(script)
		io.WriteString(dst, "echo out\n")
		dst.Close()

		cmd := testexec.CommandContext(ctx, shell, script)
		out, err := cmd.CombinedOutput()
		if exec_path {
			// Scripts on exec paths should always work.
			if err != nil {
				s.Errorf("%v [%v] failed: %v", shell, script, err)
				cmd.DumpLog(ctx)
				return
			}

			if string(out) != "out\n" {
				s.Errorf("%v [%v] output is incorrect: %v", shell, script, out)
				cmd.DumpLog(ctx)
				return
			}
		} else {
			// Scripts on noexec paths should either fail or warn.
			if should_fail {
				if err == nil {
					s.Errorf("%v [%v] incorrectly exited non-zero: %v", shell, script, out)
					cmd.DumpLog(ctx)
					return
				}
			} else {
			}
		}
	}

	runTestCase := func(exec_path bool, path string) {
		runShell("/bin/sh", true, exec_path, path)
		runShell("/bin/dash", true, exec_path, path)
		runShell("/bin/bash", false, exec_path, path)
	}

	// These paths should work (exec).
	for _, path := range []string{
		"/", "/usr/local",
	} {
		runTestCase(true, path)
	}

	// These paths should fail (noexec).
	for _, path := range []string{
		"/dev", "/dev/shm",
		"/home", "/home/chronos", "/home/root",
		"/media",
		"/mnt/stateful_partition",
		"/run", "/run/lock",
		"/tmp",
		"/var", "/var/run", "/var/lock", "/var/log", "/var/tmp",
	} {
		runTestCase(false, path)
	}
}
