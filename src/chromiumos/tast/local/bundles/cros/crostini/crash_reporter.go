// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"os/exec"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrashReporter,
		Desc:         "Check that crashes inside the VM produce crash reports",
		Contacts:     []string{"sidereal@google.com", "mutexlox@google.com"},
		SoftwareDeps: []string{"chrome", "metrics_consent", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:      "artifact",
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			ExtraData: []string{crostini.ImageArtifact},
		}, {
			Name:    "download",
			Pre:     crostini.StartedByDownload(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
	})
}

// checkExitError verifies that the input error is the expected error for a VM process killed with SIGABRT.
func checkExitError(err error) error {
	if err == nil {
		return errors.New("crashing process exited without error")
	}

	exitError, ok := err.(*exec.ExitError)
	if !ok {
		return errors.Wrap(err, "got wrong error type from command")
	}

	waitStatus := exitError.Sys().(syscall.WaitStatus)
	if syscall.Signal(waitStatus.ExitStatus()) != syscall.SIGABRT {
		return errors.Wrap(err, "process failed for non-SIGABRT reason")
	}

	return nil
}

func CrashReporter(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(pre.Chrome)); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer crash.TearDownCrashTest()

	oldFiles, err := crash.GetCrashes(crash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	// Trigger a crash in the root namespace of the VM
	cmd := pre.Container.VM.Command(ctx, "python3", "-c", "import os\nos.abort()")
	// Reverse the usual error checking pattern because this
	// command is supposed to crash. Instead we check that the right
	// error was encountered.
	if err := checkExitError(cmd.Run()); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to trigger crash in VM: ", err)
	}
	s.Log("Triggered a crash in the VM")

	if _, err := crash.WaitForCrashFiles(ctx, []string{crash.UserCrashDir}, oldFiles, []string{`.*\.meta`, `.*\.dmp`}); err != nil {
		s.Error("Couldn't find expected files: ", err)
	}

	// TODO(crbug.com/703926): Check that the values in the
	// metadata file make sense. In paticular, check that the guest
	// OS release (e.g. "buster") is available and that the board is
	// set to "tatl" or "tael".
}
