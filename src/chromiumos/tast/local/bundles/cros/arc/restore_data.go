// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/testing"
)

const (
	androidDataDirPath = "/opt/google/containers/android/rootfs/android-data/data"
)

// rootDataDirPathSelector returns root Android data directory. Note, returned path is a host path.
func rootDataDirPathSelector(ctx context.Context) (string, error) {
	return androidDataDirPath, nil
}

// anyChildDataAppPathSelector returns any available child entry in Android /data/app directory.
// /data/app/ content is created when installing some app from external. For in-lab environmement
// this might be any app from PlayAutoInstall list or PAI configuration apk itself. In addition
// this test does not restrict app update, so GMS Core or Play Store app update may also be
// installed in this folder.
// Note, returned path is a host path.
func anyChildDataAppPathSelector(ctx context.Context) (string, error) {
	appPath := path.Join(androidDataDirPath, "app")
	resultPath := ""
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		entries, err := ioutil.ReadDir(appPath)
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(entries) == 0 {
			return errors.Errorf("%s is still empty", appPath)
		}
		resultPath = path.Join(appPath, entries[0].Name())
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
		return "", errors.Wrapf(err, "failed to wait for any child in %s", appPath)
	}
	return resultPath, nil
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RestoreData,
		Desc:         "This verifies SELinux data restore flow in case Android /data folder has corrupted SELinux contexts",
		LacrosStatus: testing.LacrosVariantUnknown,

		Contacts: []string{
			"khmel@chromium.org", // original author
			"arc-core@google.com",
		},
		Attr: []string{"group:crosbolt", "crosbolt_perbuild"},
		// TODO(b/210155681) enable this for ARCVM once supported.
		SoftwareDeps: []string{"chrome", "chrome_internal", "android_p"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name: "whole_data",
			Val:  rootDataDirPathSelector,
		}, {
			Name: "app_data",
			Val:  anyChildDataAppPathSelector,
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

	rootPath, err := s.Param().(func(context.Context) (string, error))(ctx)
	if err != nil {
		s.Fatal("Failed to select root path: ", err)
	}

	// Damage SELinux contexts.
	testing.ContextLogf(ctx, "Damaging SELinux contexts for %s", rootPath)
	if err := testexec.CommandContext(ctx,
		"chcon", "-R", "u:object_r:cros_home_shadow_uid:s0",
		rootPath).Run(testexec.DumpLogOnError); err != nil {
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
		chrome.GAIALoginPool(credPool),
	}

	testing.ContextLog(ctx, "Create initial Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	testing.ContextLog(ctx, "ARC is not enabled, perform optin")
	maxAttempts := 2
	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		return nil, errors.Wrap(err, "failed to optin")
	}

	// Let device setup itself for some time. This includes such first party services and apps
	// like Play Store, GMS Core. Note, we are running in environment close to end user setup
	// and this may take some time. Faling waiting for idle is not an error for this test.
	testing.ContextLog(ctx, "Waiting for device is idle")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		testing.ContextLog(ctx, "Could not wait CPU is idle for the initial setup but continue")
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

	timeStart := time.Now()

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

	duration := time.Now().Sub(timeStart)

	if metric.TotalCount() != 1 {
		return errors.Wrapf(err, "unexpected histogram count for %s, got: %d, want: 1", statusMetricName, metric.TotalCount())
	}

	// 0 - not need to restore, 1 - successfully restored.
	wantSum := int64(0)
	if dataRestoreExpected {
		wantSum = 1
	}

	if metric.Sum != wantSum {
		switch metric.Sum {
		case 0:
			return errors.New("/data restoration was not done when it was expected")
		case 1:
			return errors.New("/data restoration was done when it was not expected")
		case 2:
			return errors.New("/data restoration failed")
		default:
			return errors.Errorf("/data restoration got unexpected status %d", metric.Sum)
		}
	}

	metric, err = metrics.WaitForHistogram(ctx, tconn, durationMetricName, 10*time.Second)
	if err != nil {
		if dataRestoreExpected {
			return errors.Wrapf(err, "failed to get %s histogram", durationMetricName)
		}
	} else if !dataRestoreExpected {
		return errors.Errorf("histogram %s is set but not expected", durationMetricName)
	} else {
		if metric.Sum < 0 {
			testing.ContextLogf(ctx, "/data restoration has negative duration %d ms", metric.Sum)
		}
		if time.Duration(metric.Sum*int64(time.Millisecond)) > duration {
			testing.ContextLogf(ctx, "/data restoration took longer than boot duration %d vs %d ms", metric.Sum, duration.Milliseconds())
		}
		testing.ContextLogf(ctx, "Data is restored in %d ms", metric.Sum)
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
