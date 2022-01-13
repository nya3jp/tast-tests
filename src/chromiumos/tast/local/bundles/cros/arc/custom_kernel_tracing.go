// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
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
			Val:               true,
		}, {
			Name:              "test",
			ExtraSoftwareDeps: []string{"arc"},
			Val:               false,
		}},
		VarDeps: []string{
			"arc.perfAccountPool",
		},
	})
}

// RegularBoot steps through multiple ARC boots.
func CustomKernelTracing(ctx context.Context, s *testing.State) {
	creds := chrome.Creds{User: "cros.arc.perf.09@gmail.com", Pass: "dm2V4-mG"}

	if init := s.Param().(bool); init {
		err := performArcInitialBoot(ctx, creds)
		if err != nil {
			s.Fatal("Failed to do initial optin: ", err)
		}
	} else {
		err := performArcRegularBoot(ctx, s.OutDir(), creds)
		if err != nil {
			s.Fatal("Failed to do testing session: ", err)
		}
	}

}

// performArcInitialBoot performs initial boot that includes ARC provisioning and returns GAIA
// credentials to use for regular boot wih preserved state.
func performArcInitialBoot(ctx context.Context, creds chrome.Creds) error {
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

func performArcRegularBoot(ctx context.Context, testDir string, creds chrome.Creds) error {
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
		if err := testexec.CommandContext(ctx, "/bin/bash", "-c", "echo 1 >/sys/kernel/debug/tracing/tracing_arc_on").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to start tracing")
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

	out := []byte("")
	if isVMEnabled {
		// Note, a.WriteFile does not work
		if err := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "echo 0 >/sys/kernel/debug/tracing/tracing_arc_on").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to stop tracing")
		}

		out, err = testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "cat /sys/kernel/debug/tracing/tracing_arc_read").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to read tracing")
		}
	} else {
		if err := testexec.CommandContext(ctx, "/bin/bash", "-c", "echo 0 >/sys/kernel/debug/tracing/tracing_arc_on").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to stop tracing")
		}

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
