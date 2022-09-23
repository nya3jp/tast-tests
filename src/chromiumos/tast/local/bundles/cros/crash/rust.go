// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Rust,
		Desc: "Test the crash signature of rust binaries using the memfd panic handler",
		Contacts: []string{
			"allenwebb@chromium.org",
			"psoberoi@google.com",
			"cros-telemetry@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func Rust(ctx context.Context, s *testing.State) {
	const executable = "/usr/local/libexec/tast/helpers/local/cros/crash.Rust.panic"
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(ctx); err != nil {
			s.Error("Failed to tear down crash test: ", err)
		}
	}()

	cmd := testexec.CommandContext(ctx, executable)
	err := cmd.Run()
	if err == nil {
		s.Fatal("Expected crash, but command exited normally")
	} else if exitError, ok := err.(*exec.ExitError); ok {
		s.Log("Rust crasher exit code: ", exitError.ProcessState.ExitCode())
	} else {
		s.Fatal("Could not start rust crasher: ", err)
	}
	pid := cmd.Cmd.Process.Pid

	pattern := fmt.Sprintf("crash_Rust_panic.*.%d.*", pid)
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
		if strings.HasSuffix(match, ".meta") {
			contents, err := ioutil.ReadFile(match)
			if err != nil {
				s.Errorf("Couldn't read meta file %s contents: %v", match, err)
				continue
			}
			found = true
			if !strings.Contains(string(contents), "sig=panicked at 'See you later, alligator!', crash.Rust.panic.rs:") {
				s.Error("Failed to find crash signature")
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), match); err != nil {
					s.Error("Failed to save the meta file: ", err)
				}
			}
		}
	}
	if !found {
		s.Error("Failed to find meta file")
	}
	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
