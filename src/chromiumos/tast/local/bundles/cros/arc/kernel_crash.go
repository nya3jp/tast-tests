// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/arccrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelCrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test handling of a guest kernel crash",
		Contacts:     []string{"kimiyuki@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name:              "real_consent",
			ExtraSoftwareDeps: []string{"metrics_consent"},
			Val:               crash.RealConsent,
		}, {
			Name: "mock_consent",
			Val:  crash.MockConsent,
		}},
	})
}

func KernelCrash(ctx context.Context, s *testing.State) {
	s.Log("Connecting to Chrome")
	// We cannot use the precondition arc.Booted(). We need to close and re-assign the ARC struct, but calling arc.New() is forbidden while using the precondition.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while (re)booting ARC: ", err)
		}
	}()

	s.Log("Starting to ARCVM")
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARCVM: ", err)
	}
	defer func() {
		if a != nil {
			a.Close(ctx)
		}
	}()

	opt := crash.WithMockConsent()
	if s.Param().(crash.ConsentType) == crash.RealConsent {
		opt = crash.WithConsent(cr)
	}

	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	// Get the PID of old ARCVM process before causing a kernel crash.
	oldPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get init PID before kernel crash: ", err)
	}

	s.Log("Making crash")
	// The user of /proc/sysrq-trigger is root, but `adb shell` uses shell user. So we need to use android-sh.
	cmd := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "echo c >/proc/sysrq-trigger")
	// The android-sh returns the exit code of `echo` command when the command fails to crash ARCVM or the command stops before the crash of ARCVM actually started. The return code seems unstable, so we use Log() instead of Error() or Fatal().
	if err := cmd.Run(testexec.DumpLogOnError); err == nil {
		s.Log("The android-sh process to crash ARCVM finished with exit code 0")
	} else {
		s.Log("The android-sh process to crash ARCVM finished with an error: ", err)
	}

	s.Log("Waiting for old ARCVM process to stop")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := process.NewProcess(oldPID); err == nil {
			return errors.Errorf("ARCVM (pid %d) still exists", oldPID)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for old ARCVM process to exit: ", err)
	}
	if err := a.Close(ctx); err != nil {
		s.Error("Failed to close a object associated with ARC: ", err)
	}

	// The crash reports are sent via Mojo from the ArcCrashCollector service in ARCVM. So we need to wait the reboot of ARCVM.
	// Also `crash.WaitForCrashFiles` waits for a while but it's too short for reboot of ARCVM.
	s.Log("Waiting for new ARCVM process to start")
	if a, err = arc.New(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to start ARCVM: ", err)
	}
	// `defer a.Close(ctx)` is not needed here because it's already declared.

	s.Log("Waiting for crash files to become present")
	// Wait files like arcvm_kernel.20200420.204845.12345.0.log in the daemon-store directory.
	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	const stem = `arcvm_kernel\.\d{8}\.\d{6}\.\d+\.\d+`
	metaFileName := stem + crash.MetadataExt
	files, err := crash.WaitForCrashFiles(ctx, crashDirs, []string{
		stem + crash.LogExt, metaFileName,
	})
	if err != nil {
		s.Fatal("Failed to find files: ", err)
	}
	defer crash.RemoveAllFiles(ctx, files)

	metaFiles, ok := files[metaFileName]
	if !ok || len(metaFiles) != 1 {
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFiles...); err != nil {
			s.Error("Failed to save unexpected crashes: ", err)
		}
		s.Fatalf("Unexpectedly saw %d crashes. Saved for debugging", len(metaFiles))
	}
	// WaitForCrashFiles guarantees that there will be a match for all regexes if it succeeds,
	// so this must exist.
	metaFile := metaFiles[0]

	s.Log("Validating the meta file")
	bp, err := arccrash.GetBuildProp(ctx, a)
	if err != nil {
		if err := arccrash.UploadSystemBuildProp(ctx, a, s.OutDir()); err != nil {
			s.Error("Failed to get build.prop: ", err)
		}
		s.Fatal("Failed to get BuildProperty: ", err)
	}
	isValid, err := arccrash.ValidateBuildProp(ctx, metaFile, bp)
	if err != nil {
		s.Fatal("Failed to validate meta file: ", err)
	}
	if !isValid {
		s.Error("validateBuildProp failed. Saving meta file")
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
			s.Error("Failed to save the meta file: ", err)
		}
	}
}
