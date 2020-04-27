// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"io/ioutil"
	"os/exec"
	"regexp"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

const (
	boardRegexp = "board=(tatl|tael)"
	osRegexp    = "upload_var_vm_os_release=.*(stretch|buster)"
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
			"cros-telemetry@google.com",
		},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			ExtraHardwareDeps: crostini.CrostiniUnstable,
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

	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent(), crash.DaemonStore()); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer crash.TearDownCrashTest(ctx, crash.TearDownDaemonStore())

	daemonStorePaths, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Failed to get daemon store crash dir: ", err)
	}

	// Trigger a crash in the root namespace of the VM
	cmd := pre.Container.VM.Command(ctx, "bash", "-c", "kill -s SIGABRT $$")
	// Reverse the usual error checking pattern because this
	// command is supposed to crash. Instead we check that the right
	// error was encountered.
	if err := checkExitError(cmd.Run()); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to trigger crash in VM: ", err)
	}
	s.Log("Triggered a crash in the VM")

	files, err := crash.WaitForCrashFiles(ctx, daemonStorePaths, []string{},
		[]string{`vm_crash.*\.meta`,
			`vm_crash.*\.dmp`,
			`vm_crash.*\.proclog`})
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	s.Log("Checking for expected metadata values")

	metaData, err := ioutil.ReadFile(files[`vm_crash.*\.meta`][0])
	if err != nil {
		s.Fatal("Failed to read metadata file: ", err)
	}
	if re := regexp.MustCompile(boardRegexp); !re.Match(metaData) {
		s.Fatalf("Did not find expected line %q in metadata file", boardRegexp)
	}
	if re := regexp.MustCompile(osRegexp); !re.Match(metaData) {
		s.Fatalf("Did not find expected line %q in metadata file", osRegexp)
	}
}
