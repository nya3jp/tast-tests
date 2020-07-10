// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StadiaCUJ,
		Desc:         "Measures the performance of critical user journey for Stadia",
		Contacts:     []string{"yichenz@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      5 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
		},
		Pre: cuj.LoggedInToCUJUser(),
	})
}

// StadiaCUJ test starts the default game 'Worm Game' and runs it for 5 minutes. It stays on the
// main game menu instead of playing or interacting with the game.
func StadiaCUJ(ctx context.Context, s *testing.State) {
	const timeout = 10 * time.Second

	cr := s.PreValue().(cuj.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	conn, err := cr.NewConn(ctx, "https://ggp-staging.sandbox.google.com")
	if err != nil {
		s.Fatal("Failed to open the stadia staging instance: ", err)
	}
	defer conn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	id0 := ws[0].ID

	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)

	// Start the worm game for testing.
	wormGame, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Worm Game Edition Play game", Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		s.Fatal("Failed to find the worm game: ", err)
	}
	defer wormGame.Release(ctx)
	wormGame.FocusAndWait(ctx, timeout)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := wormGame.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click the worm game start button")
		}
		w0, err := ash.GetWindow(ctx, tconn, id0)
		if err != nil {
			s.Fatal("Failed to get the window: ", err)
		}
		// The window should turn into fullscreen mode when game starts.
		if w0.State != ash.WindowStateFullscreen {
			return errors.New("hasn't entered the game yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatal("Failed to start the worm game: ", err)
	}

	// Wait for the game screen to show up.
	// TODO(crbug.com/1091976): use signal from Stadia games instead.
	if err := testing.Sleep(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	configs := []cuj.MetricConfig{cuj.NewCustomMetricConfig(
		"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
		"percent", perf.SmallerIsBetter, []int64{50, 80})}

	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	if err := recorder.Run(ctx, tconn, func() error {
		if err := kb.Accel(ctx, "Space"); err != nil {
			return errors.Wrap(err, "failed to enter the menu")
		}
		// Run the game for 30 seconds.
		if err := testing.Sleep(ctx, 30*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	// Check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the perf data: ", err)
	}
}
