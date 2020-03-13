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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrashReporter,
		Desc: "Check that crashes inside the VM produce crash reports",
		Contacts: []string{
			// Crostini
			"sidereal@google.com",
			"cros-containers-dev@google.com",
			// Monitoring and forensics
			"mutexlox@google.com",
			"cros-monitoring-forensics@google.com",
		},
		SoftwareDeps: []string{"chrome", "metrics_consent", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			ExtraSoftwareDeps: []string{"crostini_stable"},
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			ExtraSoftwareDeps: []string{"crostini_unstable"},
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
	defer crash.TearDownCrashTest(ctx)

	daemonStorePath, err := crash.GetDaemonStoreCrashDir(ctx)
	if err != nil {
		s.Fatal("Failed to get daemon store crash dir: ", err)
	}

	oldFiles, err := crash.GetCrashes(daemonStorePath)
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

	files, err := crash.WaitForCrashFiles(ctx, []string{daemonStorePath}, oldFiles, []string{`vm_crash.*\.meta`, `vm_crash.*\.dmp`, `vm_crash.*\.proclog`})
	if err != nil {
		s.Error("Couldn't find expected files: ", err)
	}

	s.Log("Checking for expected metadata values")

	metaFilePath := daemon_store_path + "/" + files[`vm_crash.*\.meta`][0]
	cmd := testexec.CommandContext(ctx, "grep", "-E", "^board=(tatl|tael)$", metaFilePath)
	if err = cmd.Run(); err != nil {
		s.Fatal("Did not find expected line 'board=(tatl|tael)' in metadata file: ", err)
	}

	cmd := testexec.CommandContext(ctx, "grep", "-E", "^upload_var_vm_os_release=.*(stretch|buster)", metaFilePath)
	if err = cmd.Run(); err != nil {
		s.Fatal("Did not find expected line 'upload_var_vm_os_release=.*(stretch|buster)' in metadata file: ", err)
	}
}
