// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RestoreData,
		Desc: "This verifies SELinux data restore flow in case Android /data folder has corrupted SELinux contextss",

		Contacts: []string{
			"khmel@chromium.org", // original author
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
			"ui.gaiaPoolDefault",
		},
	})
}

// RestoreData steps through three ARC boots, initial provisioning and two regular logins.
// Android data folder is corrupted after initial provisioning and first regular boot should
// restore it. Second regular boot is done using recoverted /data and no restore data should
// happen.
func RestoreData(ctx context.Context, s *testing.State) {
	creds, err := restoreDataInitialBoot(ctx, s.RequiredVar("ui.gaiaPoolDefault"))
	if err != nil {
		s.Fatal("Failed to do initial optin: ", err)
	}

	// Damage SELinux contexts.
	if err := testexec.CommandContext(ctx,
		"chcon", "-R", "u:object_r:cros_home_shadow_uid:s0",
		"/opt/google/containers/android/rootfs/android-data/data/dalvik-cache").Run(testexec.DumpLogOnError); err != nil {
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

func restoreDataInitialBoot(ctx context.Context, credPool string) (*chrome.Creds, error) {
	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALoginPool(credPool),
	}

	testing.ContextLog(ctx, "Create initial Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test connection")
	}

	testing.ContextLog(ctx, "ARC is not enabled, perform optin")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to optin")
	}

	// Let device setup itself for some time. There is no clear trigger to wait so
	// wait predefined time.
	testing.ContextLog(ctx, "Waiting for the device setup")
	if err := testing.Sleep(ctx, 3*time.Minute); err != nil {
		return nil, errors.Wrap(err, "failed to wait for the device setup")
	}

	creds := cr.Creds()
	return &creds, nil
}

// restoreDataRegularBoot performs ARC boot and waits data restore metrics are available.
func restoreDataRegularBoot(ctx context.Context, testDir string, creds *chrome.Creds, dataRestoreExpected bool) error {
	const (
		statusMetricName   = "Arc.DataRestore.Status"
		durationMetricName = "Arc.DataRestore.Duration"
	)

	// Drop caches to simulate cold start when data not in system caches already.
	if err := disk.DropCaches(ctx); err != nil {
		return errors.Wrap(err, "failed to drop caches")
	}

	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALogin(*creds),
		chrome.KeepState(),
	}

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

	metric, err := metrics.WaitForHistogram(ctx, tconn, statusMetricName, 5*time.Minute)
	if err != nil {
		return errors.Wrapf(err, "failed to get %s histogram", statusMetricName)
	}

	if metric.TotalCount() != 1 {
		return errors.Wrapf(err, "unexpected histogram count for %s, got: %d, want: 1", statusMetricName, metric.TotalCount())
	}

	// 0 - not need to restore, 1 - successfully restored.
	wantSum := int64(0)
	if dataRestoreExpected {
		wantSum = 1
	}

	if metric.Sum != wantSum {
		return errors.Wrapf(err, "unexpected histogram sum for %s, got: %d, want: %d", statusMetricName, metric.Sum, wantSum)
	}

	metric, err = metrics.WaitForHistogram(ctx, tconn, durationMetricName, 10*time.Second)
	if err != nil {
		if dataRestoreExpected {
			return errors.Wrapf(err, "failed to get %s histogram", durationMetricName)
		}
		testing.ContextLogf(ctx, "Data is restored in %d ms", metric.Sum)
	} else if !dataRestoreExpected {
		return errors.Errorf("histogram %s is set but not expected", durationMetricName)
	}

	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, 3*time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait Play Store shown")
	}

	testing.ContextLog(ctx, "Test app was successfully started")

	return nil
}
