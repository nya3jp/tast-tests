// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/testing"
	"chromiumos/tast/common/testexec"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RestoreData,
		Desc: "This verifies SELinux data restore",

		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-core@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               []string{},
		}},
		VarDeps: []string{
			"arc.perfAccountPool",
		},
	})
}

// RestoreData steps through two ARC boots, initial provisioning and regular login.
// Android data folder is corrupted between logins.
func RestoreData(ctx context.Context, s *testing.State) {
	creds, err := restoreDataInitialBoot(ctx, s.RequiredVar("arc.perfAccountPool"))
	if err != nil {
		s.Fatal("Failed to do initial optin: ", err)
	}
	
	// Damage SELinux context
	if err := testexec.CommandContext(ctx, "chcon", "-R", "u:object_r:cros_home_shadow_uid:s0", "/opt/google/containers/android/rootfs/android-data/data").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to damage SELinux contexts: ", err)
	}

	testing.ContextLog(ctx, "Verify boot that requires data restore")
	err = restoreDataRegularBoot(ctx, s.OutDir(), creds, true)
	if err != nil {
		s.Fatal("Failed to do regular boot with data restore: ", err)
	}

	testing.ContextLog(ctx, "Verify boot that does not require data restore")
	err = restoreDataRegularBoot(ctx, s.OutDir(), creds, false)
	if err != nil {
		s.Fatal("Failed to do regular boot without data restore: ", err)
	}
}

func restoreDataInitialBoot(ctx context.Context, credPool string) (chrome.Creds, error) {
	// Options are tuned for the fastest boot, we don't care about
	// initial provisioning performance, which is monitored in other tests.
	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALoginPool(credPool)}

	testing.ContextLog(ctx, "Create initial Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return chrome.Creds{}, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return chrome.Creds{}, errors.Wrap(err, "failed to create test connection")
	}

	testing.ContextLog(ctx, "ARC is not enabled, perform optin")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return chrome.Creds{}, errors.Wrap(err, "failed to optin")
	}
	
	// Let devive setup itself.
	testing.ContextLog(ctx, "Waiting for the device setup")
	time.Sleep(5 * time.Minute);

	return cr.Creds(), nil
}

// performArcRegularBoot performs ARC boot and waits data restore metrics are available.
func restoreDataRegularBoot(ctx context.Context, testDir string, creds chrome.Creds, dataRestoreExpected bool) error {

	// Drop caches to simulate cold start when data not in system caches already.
	if err := disk.DropCaches(ctx); err != nil {
		return errors.Wrap(err, "failed to drop caches")
	}

	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALogin(creds),
		chrome.KeepState()}

	testing.ContextLog(ctx, "Create Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, testDir)
	if err != nil {
		return errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close(ctx)

	waitCtx, cancel := context.WithTimeout(ctx, 2 * time.Minute)
	defer cancel()

	predResume := arc.RegexpPred(regexp.MustCompile(`Resuming normal boot`))
	if err := a.WaitForLogcat(waitCtx, predResume); err != nil {
		return errors.Wrap(err, "failed to wait ARC is resumed booting")
	}


	output, err := a.Command(ctx, "logcat", "-d").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to read logcat")
	}
	
	outputStr := string(output)
	failedToRestore := regexp.MustCompile(`data secontexts are not restored in`)
	m := failedToRestore.FindStringSubmatch(outputStr)
	if m != nil {
		return errors.Errorf("Found that data restore failed")
	}

	okToRestore := regexp.MustCompile(`data secontexts are restored in`)
	m = okToRestore.FindStringSubmatch(outputStr)
	if m != nil {
		if dataRestoreExpected {
			testing.ContextLog(ctx, "Data restore happened and this is expected")
		} else {
			return errors.Errorf("Data restore happened and this is not expected")
		}
	} else {
		if !dataRestoreExpected {
			testing.ContextLog(ctx, "Data restore did not happen and this is expected")
		} else {
			return errors.Errorf("Data restore did not happen and this is not expected")
		}
	}


	return nil
}
