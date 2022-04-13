// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosperf implements a library used for utilities for running perf tests with lacros.
package lacrosperf

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
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
	// Set-up environment to be more consistent.
	sup, supCleanup := setup.New(name)
	cleanup := CombineCleanup(ctx, noCleanup, supCleanup, "failed to clean up perf test")
	defer func() {
		if retErr != nil {
			cleanup(ctx)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{Wifi: setup.DoNotChangeWifiInterfaces, NightLight: setup.DisableNightLight}))

	if err := sup.Check(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to setup power test environment")
	}

	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodDoNotDisturb, true); err != nil {
		return nil, errors.Wrap(err, "failed to disable notifications")
	}
	cleanup = CombineCleanup(ctx, cleanup, func(ctx context.Context) error {
		return quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodDoNotDisturb, false)
	}, "failed to re-enable notifications")

	// Disable automation feature for performance test.
	// ResetAutomation should be already called previously, but automation is implicitly enabled by
	// quicksettings.ToggleSetting, so we ensure it is disabled by calling ResetAutomation again.
	// TODO(b/199815100): Call private API to block accessibility features here.
	if err := tconn.ResetAutomation(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reset the automation feature")
	}

	return cleanup, nil
}

// cooldownConfig is the configuration used to wait for the stabilization of CPU
// shared between ash-chrome test setup and lacros-chrome test setup.
var cooldownConfig = cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)

// StabilizeCondition describes what condition to use in lacros perftests to
// stabilize the environment.
type StabilizeCondition int

const (
	// StabilizeBeforeOpeningURL indicates that we should wait for e.g. CPU stability
	// before opening the URL. Use this if your page actively uses resources, i.e. the CPU
	// could not reach stability while your page is open.
	StabilizeBeforeOpeningURL = iota
	// StabilizeAfterOpeningURL indicates that we should wait for e.g. CPU stability
	// after opening the URL. Use this if your page is relatively static and CPU stability
	// can be reached. This option is preferable, if possible.
	StabilizeAfterOpeningURL
)

// SetupCrosTestWithPage opens a cros-chrome page after waiting for a stable environment (CPU temperature, etc).
func SetupCrosTestWithPage(ctx context.Context, cr *chrome.Chrome, url string, stabilize StabilizeCondition) (*chrome.Conn, CleanupCallback, error) {
	// Depending on the page, opening it may cause continuous CPU usage (e.g. WebGL aquarium),
	// so wait until stabilized before opening the tab if we are instructed to do so.
	if stabilize == StabilizeBeforeOpeningURL {
		if err := cpu.WaitUntilStabilized(ctx, cooldownConfig); err != nil {
			return nil, nil, err
		}
	}

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open new tab")
	}

	cleanup := func(ctx context.Context) error {
		conn.CloseTarget(ctx)
		conn.Close()
		return nil
	}

	// For some tests, it is safe to wait for stabilization after opening the tab.
	if stabilize == StabilizeAfterOpeningURL {
		if err := cpu.WaitUntilStabilized(ctx, cooldownConfig); err != nil {
			if cerr := cleanup(ctx); cerr != nil {
				testing.ContextLog(ctx, "Failed to clean up: ", cerr)
			}
			return nil, nil, err
		}
	}

	return conn, cleanup, nil
}

// SetupLacrosTestWithPage opens a lacros-chrome page after waiting for a stable environment (CPU temperature, etc).
func SetupLacrosTestWithPage(ctx context.Context, cr *chrome.Chrome, url string, stabilize StabilizeCondition) (
	retConn *chrome.Conn, retTConn *chrome.TestConn, retL *lacros.Lacros, retCleanup CleanupCallback, retErr error) {
	// Launch lacros-chrome with about:blank loaded first - we don't want to include startup cost.
	// Since we also want to wait until the CPU is stabilized as much as possible,
	// we first open with about:blank to remove startup cost as a variable as much as possible.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to connect to test API")
	}

	l, err := lacros.LaunchWithURL(ctx, tconn, chrome.BlankURL)
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

	// Depending on the page, opening it may cause continuous CPU usage (e.g. WebGL aquarium),
	// so wait until stabilized before opening the tab if we are instructed to do so.
	if stabilize == StabilizeBeforeOpeningURL {
		if err := cpu.WaitUntilStabilized(ctx, cooldownConfig); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	// If we are opening about:blank, then we re-use the existing page that opened when we launched lacros.
	// If not, we navigate to the specified page.
	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL(chrome.BlankURL))
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to open new tab")
	}
	cleanup = CombineCleanup(ctx, cleanup, func(ctx context.Context) error {
		conn.CloseTarget(ctx)
		conn.Close()
		return nil
	}, "")

	// If we want about:blank, don't close the initial about:blank we opened.
	// Otherwise, close the initial "about:blank" tab present at startup.
	if url != chrome.BlankURL {
		if err := conn.Navigate(ctx, url); err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "failed to navigate to url")
		}
	}

	// For some specific tests, it is safe to wait for stabilization after opening the tab.
	if stabilize == StabilizeAfterOpeningURL {
		if err := cpu.WaitUntilStabilized(ctx, cooldownConfig); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	return conn, ltconn, l, cleanup, nil
}
