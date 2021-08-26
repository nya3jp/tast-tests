// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements a library used for utilities and communication with lacros-chrome on ChromeOS.
package lacros

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
)

// CleanupCallback is a callback that should be deferred to clean up test resources.
type CleanupCallback func(context.Context) error

func noCleanup(context.Context) error { return nil }

// CombineCleanup combines two CleanupCallbacks so they are executed in the same order
// that they would be if they had been deferred.
func CombineCleanup(ctx context.Context, existing, new func(context.Context) error, msg string) CleanupCallback {
	return func(context.Context) error {
		if err := new(ctx); err != nil {
			existing(ctx)
			return errors.Wrap(err, msg)
		}
		return existing(ctx)
	}
}

// SetupPerfTest sets up a stable environment for a lacros performance test.
// The returned CleanupCallback should be deferred to be executed upon test completion.
func SetupPerfTest(ctx context.Context, tconn *chrome.TestConn, name string) (retCleanup CleanupCallback, retErr error) {
	// Set-up environment to be more consistent:
	sup, supCleanup := setup.New(name)
	cleanup := CombineCleanup(ctx, noCleanup, supCleanup, "failed to clean up perf test")
	defer func() {
		if retErr != nil {
			cleanup(ctx)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{Wifi: setup.DoNotChangeWifiInterfaces, NightLight: setup.DisableNightLight}))

	if err := sup.Check(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to setup GpuCUJ power test environment")
	}

	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodDoNotDisturb, true); err != nil {
		return nil, errors.Wrap(err, "failed to disable notifications")
	}
	cleanup = CombineCleanup(ctx, cleanup, func(ctx context.Context) error {
		return quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodDoNotDisturb, false)
	}, "failed to re-enable notifications")

	return cleanup, nil
}

func waitForStableEnvironment(ctx context.Context) error {
	// Wait for CPU to cool down.
	if _, err := power.WaitUntilCPUCoolDown(ctx, power.DefaultCoolDownConfig(power.CoolDownPreserveUI)); err != nil {
		return errors.Wrap(err, "failed to wait for CPU to cool down")
	}

	// Wait for quiescent state.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}
	return nil
}

// SetupCrosTestWithPage opens a cros-chrome page after waiting for a stable environment (CPU temperature, etc).
func SetupCrosTestWithPage(ctx context.Context, f launcher.FixtData, url string) (*chrome.Conn, CleanupCallback, error) {
	if err := waitForStableEnvironment(ctx); err != nil {
		return nil, nil, err
	}

	conn, err := f.Chrome.NewConn(ctx, url)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open new tab")
	}
	return conn, func(ctx context.Context) error {
		conn.CloseTarget(ctx)
		conn.Close()
		return nil
	}, nil
}

// SetupLacrosTestWithPage opens a lacros-chrome page after waiting for a stable environment (CPU temperature, etc).
func SetupLacrosTestWithPage(ctx context.Context, f launcher.FixtData, url string) (
	retConn *chrome.Conn, retTConn *chrome.TestConn, retL *launcher.LacrosChrome, retCleanup CleanupCallback, retErr error) {
	// Launch lacros-chrome with about:blank loaded first - we don't want to include startup cost.
	l, err := launcher.LaunchLacrosChrome(ctx, f)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
	}
	cleanup := func(ctx context.Context) error {
		l.Close(ctx)
		return nil
	}
	defer func() {
		if retErr != nil {
			cleanup(ctx)
		}
	}()

	ltconn, err := l.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to connect to test API")
	}

	if err := waitForStableEnvironment(ctx); err != nil {
		return nil, nil, nil, nil, err
	}

	conn, err := l.NewConn(ctx, url)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to open new tab")
	}
	cleanup = CombineCleanup(ctx, cleanup, func(ctx context.Context) error {
		conn.CloseTarget(ctx)
		conn.Close()
		return nil
	}, "")

	// Close the initial "about:blank" tab present at startup.
	if err := CloseAboutBlank(ctx, f.TestAPIConn, l.Devsess, 0); err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to close about:blank tab")
	}

	return conn, ltconn, l, cleanup, nil
}
