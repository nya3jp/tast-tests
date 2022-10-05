// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/perfboot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RegularBoot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "This exercises a scenario when user logs in Chrome where ARC is already provisioned and tries to use ARC app immediately. App launch delay is reported for this case. This does not do acutual reboot however drops caches before each iteration to match the cold start scenario",

		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      25 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		VarDeps: []string{
			"arc.perfAccountPool",
		},
	})
}

// RegularBoot steps through multiple ARC boots.
func RegularBoot(ctx context.Context, s *testing.State) {
	creds, err := performArcInitialBoot(ctx, s.RequiredVar("arc.perfAccountPool"))
	if err != nil {
		s.Fatal("Failed to do initial optin: ", err)
	}

	const iterationCount = 5
	perfValues := perf.NewValues()
	for i := 0; i < iterationCount; i++ {
		appLaunchDuration, appShownDuration, enabledScreenDuration, err := performArcRegularBoot(ctx, s.OutDir(), creds)
		if err != nil {
			s.Fatal("Failed to do regular boot: ", err)
		}

		perfValues.Append(perf.Metric{
			Name:      "app_launch_time",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, appLaunchDuration.Seconds())
		perfValues.Append(perf.Metric{
			Name:      "app_shown_time",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, appShownDuration.Seconds())
		perfValues.Append(perf.Metric{
			Name:      "boot_progress_enable_screen",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, enabledScreenDuration.Seconds())
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// performArcInitialBoot performs initial boot that includes ARC provisioning and returns GAIA
// credentials to use for regular boot wih preserved state.
func performArcInitialBoot(ctx context.Context, credPool string) (chrome.Creds, error) {
	// Options are tuned for the fastest boot, we don't care about
	// initial provisioning performance, which is monitored in other tests.
	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.GAIALoginPool(credPool),
		chrome.ExtraArgs(arc.DisableSyncFlags()...)}

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

	if err := optin.WaitForPlayStoreShown(ctx, tconn, 2*time.Minute); err != nil {
		return chrome.Creds{}, errors.Wrap(err, "failed to wait Play Store shown")
	}

	return cr.Creds(), nil
}

// performArcRegularBoot performs ARC boot and starts Play Store app deferred and waits it is
// actually shown. It returns:
//   - time between the user session is created and and Play Store window is shown.
//   - time to fully start Android system server. This is included into the metric above.
//
// Note, it is not actually possible to measure this time directly from test due to tast
// login is complex and ends after user session is actually created. Instead it uses existing ARC
// histogram first app launch request and delay. Combined value is actual time that representss
// the hardest case when user stars ARC app immideatly after login. First app launch request
// represents here the overhead from tast Chrome login implementation.
// This also resets system caches before login to simulate scenario when user uses Chromebook after
// reboot.
// TODO (khmel): Change return value as a struct.
func performArcRegularBoot(ctx context.Context, testDir string, creds chrome.Creds) (time.Duration, time.Duration, time.Duration, error) {
	// Use custom cooling config that is bit relaxed from default implementation
	// in order to reduce failure rate especially on AMD low-end devices.
	coolDownConfig := cpu.CoolDownConfig{
		PollTimeout:          300 * time.Second,
		PollInterval:         2 * time.Second,
		TemperatureThreshold: 52000,
		CoolDownMode:         cpu.CoolDownStopUI,
	}

	if _, err := cpu.WaitUntilCoolDown(ctx, coolDownConfig); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to wait until CPU is cooled down")
	}

	// Drop caches to simulate cold start when data not in system caches already.
	if err := disk.DropCaches(ctx); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to drop caches")
	}

	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.GAIALogin(creds),
		chrome.KeepState(),
		chrome.ExtraArgs(append(arc.DisableSyncFlags())...)}

	testing.ContextLog(ctx, "Create Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to create test connection")
	}

	testing.ContextLog(ctx, "Starting Play Store window deferred")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to launch Play Store")
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, 2*time.Minute); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to wait Play Store shown")
	}

	delay, err := readFirstAppLaunchHistogram(ctx, tconn, "Arc.FirstAppLaunchDelay.TimeDelta")
	if err != nil {
		return 0, 0, 0, err
	}

	request, err := readFirstAppLaunchHistogram(ctx, tconn, "Arc.FirstAppLaunchRequest.TimeDelta")
	if err != nil {
		return 0, 0, 0, err
	}

	delayShown, err := readFirstAppLaunchHistogram(ctx, tconn, "Arc.FirstAppLaunchDelay.TimeDeltaUntilAppLaunch")
	if err != nil {
		return 0, 0, 0, err
	}

	a, err := arc.New(ctx, testDir)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to connect to ARC")
	}
	p, err := perfboot.GetPerfValues(ctx, tconn, a)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to extract ARC boot metrics")
	}

	return request + delay, request + delayShown, p["boot_progress_enable_screen"], nil
}

// readFirstAppLaunchHistogram reads histogram and converts it to Duration.
func readFirstAppLaunchHistogram(ctx context.Context, tconn *chrome.TestConn, name string) (time.Duration, error) {
	metric, err := metrics.WaitForHistogram(ctx, tconn, name, 20*time.Second)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get %s histogram", name)
	}

	timeMs, err := metric.Mean()
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read %s histogram", name)
	}

	return time.Duration(timeMs * float64(time.Millisecond)), nil
}
