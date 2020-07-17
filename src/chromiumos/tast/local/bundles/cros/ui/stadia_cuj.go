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
	"chromiumos/tast/local/chrome"
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
	const (
		timeout  = 10 * time.Second
		gameName = "Worm Game Edition"
	)

	cr := s.PreValue().(cuj.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	conn, err := cr.NewConn(ctx, "https://ggp-staging.sandbox.google.com/store/list")
	if err != nil {
		s.Fatal("Failed to open the stadia staging instance: ", err)
	}
	defer conn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)

	if err := StartGameFromGameListsView(ctx, tconn, webview, gameName, timeout); err != nil {
		s.Fatalf("Failed to start the game %s: %s", gameName, err)
	}

	// Wait for the game to be completely loaded
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

	recorder, err := cuj.NewRecorder(ctx, configs...)
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

// StartGameFromGameListsView locates the game by its name from the game list page
// and starts the game.
func StartGameFromGameListsView(ctx context.Context, tconn *chrome.TestConn, n *ui.Node, name string, timeout time.Duration) error {
	gameView := "View " + name + "."
	gamePlay := "Play"
	gameStart := name + " Play game"

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	id0 := ws[0].ID

	// Find the game view from the game list.
	gameViewButton, err := n.DescendantWithTimeout(ctx, ui.FindParams{Name: gameView, Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find the game view button (%s)", gameView)
	}
	defer gameViewButton.Release(ctx)
	gameViewButton.FocusAndWait(ctx, timeout)
	if err := gameViewButton.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click the game view button (%s)", gameView)
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for the page to be stable")
	}
	// Play the game.
	gamePlayButton, err := n.DescendantWithTimeout(ctx, ui.FindParams{Name: gamePlay, Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find the game play button (%s)", gamePlay)
	}
	defer gamePlayButton.Release(ctx)
	gamePlayButton.FocusAndWait(ctx, timeout)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := gamePlayButton.LeftClick(ctx); err != nil {
			return errors.Wrapf(err, "failed to click the game play button (%s)", gamePlay)
		}
		startButtonExists, err := n.DescendantExists(ctx, ui.FindParams{Name: gameStart, Role: ui.RoleTypeButton})
		if err != nil {
			return errors.Wrap(err, "failed to check if start button exists")
		}
		if startButtonExists != true {
			return errors.New("start button doesn't exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to show the game start button for %s", name)
	}
	// Start(enter) the game.
	gameStartButton, err := n.DescendantWithTimeout(ctx, ui.FindParams{Name: gameStart, Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find the game start button (%s)", gameStart)
	}
	defer gameStartButton.Release(ctx)
	gameStartButton.FocusAndWait(ctx, timeout)
	// Make sure the game is fully launched.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := gameStartButton.LeftClick(ctx); err != nil {
			return errors.Wrapf(err, "failed to click the game start button (%s)", gameStart)
		}
		w0, err := ash.GetWindow(ctx, tconn, id0)
		if err != nil {
			return errors.Wrapf(err, "failed to get the window")
		}
		// The window should turn into fullscreen mode when game starts.
		if w0.State != ash.WindowStateFullscreen {
			return errors.New("hasn't entered the game yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to start the game %s", name)
	}
	return nil
}
