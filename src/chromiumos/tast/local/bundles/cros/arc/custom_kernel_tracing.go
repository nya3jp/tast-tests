// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/tracing"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CustomKernelTracing,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Desc",

		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name:              "init",
			ExtraSoftwareDeps: []string{"arc"},
			Val:               0,
		}, {
			Name:              "regular_boot",
			ExtraSoftwareDeps: []string{"arc"},
			Val:               1,
		}, {
			Name:              "game_load",
			ExtraSoftwareDeps: []string{"arc"},
			Val:               2,
		}},
		VarDeps: []string{
			"arc.perfAccountPool",
		},
	})
}

// CustomKernelTracing performs various testing flows.
func CustomKernelTracing(ctx context.Context, s *testing.State) {
	creds := chrome.Creds{User: "cros.arc.perf.09@gmail.com", Pass: "dm2V4-mG"}

	mode := s.Param().(int)

	if mode == 0 {
		err := tracingInitialBoot(ctx, creds)
		if err != nil {
			s.Fatal("Failed to do initial optin: ", err)
		}
	} else if mode == 1 {
		err := tracingRegularBoot(ctx, s.OutDir(), creds)
		if err != nil {
			s.Fatal("Failed to do regular boot testing session: ", err)
		}
	} else if mode == 2 {
		err := tracingGameLoading(ctx, s.OutDir(), creds)
		if err != nil {
			s.Fatal("Failed to do game testing session: ", err)
		}
	}

}

func tracingInitialBoot(ctx context.Context, creds chrome.Creds) error {
	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.GAIALogin(creds)}

	testing.ContextLog(ctx, "Create initial Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test connection")
	}

	testing.ContextLog(ctx, "ARC is not enabled, perform optin")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to optin")
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait Play Store shown")
	}

	return nil
}

func tracingArcOn(ctx context.Context, cmd string) error {
	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to check if VM is running")
	}

	if isVMEnabled {
		if err := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "echo "+cmd+" >/sys/kernel/debug/tracing/tracing_arc_on").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to access ARC tracing")
		}
	} else {
		if err := testexec.CommandContext(ctx, "/bin/bash", "-c", "echo "+cmd+" >/sys/kernel/debug/tracing/tracing_arc_on").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to acces ARC tracing")
		}
	}

	return nil
}

func tracingStopAndPrintResult(ctx context.Context, testDir string) error {
	if err := tracingArcOn(ctx, "0"); err != nil {
		return err
	}

	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to check if VM is running")
	}

	out := []byte("")
	if isVMEnabled {
		out, err = testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "cat /sys/kernel/debug/tracing/tracing_arc_read").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to read tracing")
		}
	} else {

		out, err = testexec.CommandContext(ctx, "/bin/bash", "-c", "cat /sys/kernel/debug/tracing/tracing_arc_read").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to read tracing")
		}
	}

	tracingPath := filepath.Join(testDir, "tracing.txt")
	if err := ioutil.WriteFile(tracingPath, out, 0644); err != nil {
		return errors.Wrap(err, "failed to write tracing")
	}
	testing.ContextLogf(ctx, "Tracing data is serialized to %s", tracingPath)

	testing.ContextLog(ctx, "***************  TOTAL ***************")
	result, err := tracing.AnylyzeTracing(string(out), "total", 0)
	if err != nil {
		return errors.Wrap(err, "failed to analyze tracing")
	}
	testing.ContextLog(ctx, result)
	testing.ContextLog(ctx, "**************  PER FS ***************")
	result, err = tracing.AnylyzeTracing(string(out), "fs", 10000)
	if err != nil {
		return errors.Wrap(err, "failed to analyze tracing")
	}
	testing.ContextLog(ctx, result)
	testing.ContextLog(ctx, "***********  PER FS AND OP ************")
	result, err = tracing.AnylyzeTracing(string(out), "fsop", 10000)
	if err != nil {
		return errors.Wrap(err, "failed to analyze tracing")
	}
	testing.ContextLog(ctx, result)
	testing.ContextLog(ctx, "***************  PER FILE *************")
	result, err = tracing.AnylyzeTracing(string(out), "file", 10000)
	if err != nil {
		return errors.Wrap(err, "failed to analyze tracing")
	}
	testing.ContextLog(ctx, result)

	return nil
}

func tracingRegularBoot(ctx context.Context, testDir string, creds chrome.Creds) error {
	chromeArgs := []string{"--arc-start-mode=manual", "--arc-disable-ureadahead"}
	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALogin(creds),
		chrome.KeepState(),
		chrome.ExtraArgs(append(arc.DisableSyncFlags(), chromeArgs...)...)}

	testing.ContextLog(ctx, "Create Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test connection")
	}

	testing.ContextLog(ctx, "Force sleeping")
	testing.Sleep(ctx, 30*time.Second)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait CPU idle")
	}

	// Drop caches to simulate cold start when data not in system caches already.
	if err := disk.DropCaches(ctx); err != nil {
		return errors.Wrap(err, "failed to drop caches")
	}

	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to check if VM is running")
	}

	if isVMEnabled {
		// It is expected custom kernel build with ARC tracing available from start.
	} else {
		if err := tracingArcOn(ctx, "1"); err != nil {
			return err
		}
	}

	if err = arc.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start ARC")
	}

	testing.ContextLog(ctx, "Starting Play Store window deferred")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait Play Store shown")
	}

	if err := tracingStopAndPrintResult(ctx, testDir); err != nil {
		return errors.Wrap(err, "failed to get tracing results")
	}

	return nil
}

// tracingLaunchAndWaitAppStarted launches and waits the app is completely loaded and is in active state.
func tracingLaunchAndWaitAppStarted(ctx context.Context,
	a *arc.ARC,
	tconn *chrome.TestConn,
	activityName, keyword string,
	timeout time.Duration) (time.Duration, error) {
	startTime := time.Now()

	testing.ContextLog(ctx, "Launching app")

	// Start activity w/ fullscreen
	if err := a.Command(ctx, "am", "start", "--windowingMode", "1", "-n", activityName).Run(testexec.DumpLogOnError); err != nil {
		return 0, errors.Wrap(err, "failed to launch app")
	}

	testing.ContextLog(ctx, "Waiting app started")

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var pred = arc.RegexpPred(regexp.MustCompile(keyword))
	if err := a.WaitForLogcat(waitCtx, pred); err != nil {
		return 0, errors.Wrap(err, "wait for app load timed out, intent wasn't found in logcat")
	}

	var loadTime = time.Now().Sub(startTime)

	return loadTime, nil
}

func tracingGameLoading(ctx context.Context, testDir string, creds chrome.Creds) error {
	var chromeArgs []string
	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALogin(creds),
		chrome.KeepState(),
		chrome.ExtraArgs(append(arc.DisableSyncFlags(), chromeArgs...)...)}
	testing.ContextLog(ctx, "Create Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test connection")
	}

	a, err := arc.New(ctx, testDir)
	if err != nil {
		return errors.Wrap(err, "failed to connect to ARC")
	}
	defer a.Close(ctx)

	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to check if VM is running")
	}

	// Stop previous tracing just in case
	if isVMEnabled {
		if err := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "echo 0 >/sys/kernel/debug/tracing/tracing_arc_on").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to stop tracing")
		}
	}

	testing.ContextLog(ctx, "Force sleeping")
	testing.Sleep(ctx, 30*time.Second)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait CPU idle")
	}

	// Drop caches to simulate cold start when data not in system caches already.
	if err := disk.DropCaches(ctx); err != nil {
		return errors.Wrap(err, "failed to drop caches")
	}

	if err := tracingArcOn(ctx, "1"); err != nil {
		return err
	}

	loadTime, err := tracingLaunchAndWaitAppStarted(
		ctx, a, tconn,
		"com.gramgames.mergedragons/com.gramgames.activity.UnityPlayerActivity",
		"DEN CHEST",
		5*time.Minute)
	if err != nil {
		return errors.Wrap(err, "failed to load game")
	}

	testing.ContextLog(ctx, "Result: Load time: ", loadTime.Seconds())

	if err := tracingStopAndPrintResult(ctx, testDir); err != nil {
		return errors.Wrap(err, "failed to get tracing results")
	}

	return nil
}
