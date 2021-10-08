// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RegularBoot,
		Desc: "This exercises a scenario when user logs in Chrome where ARC is already provisioned and tries to use ARC app immediately. App launch delay is reported for this case",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      15 * time.Minute,
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

// RegularBoot steps through multiple ARC boots
func RegularBoot(ctx context.Context, s *testing.State) {
	gaia := chrome.GAIAFixedCredsFromLoginPool(s.RequiredVar("arc.perfAccountPool"))
	if _, err := performArcBoot(ctx, gaia, true /* initial */); err != nil {
		s.Fatalf("Failed to do initial optin: %q", err)
	}

	const iterationCount = 4
	perfValues := perf.NewValues()
	for i := 0; i < iterationCount; i++ {
		duration, err := performArcBoot(ctx, gaia, false /* initial */)
		if err != nil {
			s.Fatalf("Failed to do regular boot: %q", err)
		}
		perfValues.Append(perf.Metric{
			Name:      "app_launch_time",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, duration.Seconds())
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// performArcBoot performs ARC boot and starts Play Store app deferred and waits it is acutally
// shown. It returns time between the user session is created and and Play Store window is shown.
// Note, it is not acutally possible to measure this time directly from test due to tast login
// is complex and ends after user session is actually created. Instead it uses existing ARC
// histogram first app launch request and delay. Combined value is actual time that represents
// the hardest case when user stars ARC app immideatly after login. First app launch request
// represents here the overhead from tast Chrome login implementation.
func performArcBoot(ctx context.Context, gaia chrome.Option, initial bool) (time.Duration, error) {
	coolDownConfig := power.CoolDownConfig{
		PollTimeout:             300 * time.Second,
		PollInterval:            2 * time.Second,
		CPUTemperatureThreshold: 52000,
		CoolDownMode:            power.CoolDownStopUI,
	}

	if _, err := power.WaitUntilCPUCoolDown(ctx, coolDownConfig); err != nil {
		return 0, errors.Wrap(err, "failed to wait until CPU is cooled down")
	}

	if err := disk.DropCaches(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to drop caches")
	}

	var opts []chrome.Option
	opts = append(opts,
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	opts = append(opts, gaia)
	if !initial {
		opts = append(opts, chrome.KeepState())
	}

	testing.ContextLog(ctx, "Create Chrome")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return 0, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create test connection")
	}
	defer tconn.Close()

	if initial {
		testing.ContextLog(ctx, "ARC is not enabled, perform optin")
		if err = optin.Perform(ctx, cr, tconn); err != nil {
			return 0, errors.Wrap(err, "failed to optin")
		}
	}

	testing.ContextLog(ctx, "Starting Play Store window deferred")
	if err = apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return 0, errors.Wrap(err, "failed to launch Play Store")
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		return 0, errors.Wrap(err, "failed to wait Play Store shown")
	}

	if initial {
		// Note, for initial start, FirstAppLaunchRequest metric is not recorded.
		return 0, nil
	}

	delay, err := readFirstAppLaunchHistogram(ctx, tconn, "Arc.FirstAppLaunchDelay.TimeDelta")
	if err != nil {
		return 0, err
	}

	request, err := readFirstAppLaunchHistogram(ctx, tconn, "Arc.FirstAppLaunchRequest.TimeDelta")
	if err != nil {
		return 0, err
	}

	return request + delay, nil
}

//readFirstAppLaunchHistogram reads histogram and converts it to Duration.
func readFirstAppLaunchHistogram(ctx context.Context, tconn *chrome.TestConn, name string) (time.Duration, error) {
	metric, err := metrics.WaitForHistogram(ctx, tconn, name, 20*time.Second)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get %s histogram", name)
	}

	timeMs, err := metric.Mean()
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read %s histogram", name)
	}

	return time.Duration(timeMs) * time.Millisecond, nil
}
