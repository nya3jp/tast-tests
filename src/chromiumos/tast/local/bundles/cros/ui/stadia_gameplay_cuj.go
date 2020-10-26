// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/stadiacuj"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StadiaGameplayCUJ,
		Desc:         "Measures the performance of critical user journey for game playing on Stadia",
		Contacts:     []string{"yichenz@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
		},
		Pre: cuj.LoggedInToCUJUser(),
	})
}

// StadiaGameplayCUJ test starts and plays a exploration scene and gathering the performance.
// The game playing is hardcoded.
func StadiaGameplayCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout  = 10 * time.Second
		gameName = "Mortal Kombat 11"
	)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr := s.PreValue().(cuj.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	configs := []cuj.MetricConfig{cuj.NewCustomMetricConfig(
		"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
		"percent", perf.SmallerIsBetter, []int64{50, 80})}

	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	conn, err := cr.NewConn(ctx, stadiacuj.StadiaAllGamesURL)
	if err != nil {
		s.Fatal("Failed to open the stadia staging instance: ", err)
	}
	defer conn.Close()
	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(closeCtx)

	if err := stadiacuj.StartGameFromGameListsView(ctx, tconn, conn, webview, gameName, timeout); err != nil {
		s.Fatalf("Failed to start the game %s: %s", gameName, err)
	}

	// Wait for the game to be completely loaded and the opening animation to be done.
	// TODO(crbug.com/1091976): use signal from Stadia games instead.
	if err := testing.Sleep(ctx, time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	gameOngoing := false
	defer func() {
		if gameOngoing {
			// Exit the game.
			if err := stadiacuj.ExitGame(closeCtx, kb, webview); err != nil {
				s.Error("Failed to exit game: ", err)
			}
		}
	}()

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Hard code the game playing routine.
		// Enter the menu.
		if err := stadiacuj.PressKeyInGame(ctx, kb, "Enter", 10*time.Second); err != nil {
			s.Fatal("Failed to enter the menu: ", err)
		}
		// Enter story mode.
		if err := stadiacuj.PressKeyInGame(ctx, kb, "Enter", time.Second); err != nil {
			s.Fatal("Failed to enter Story Mode: ", err)
		}
		// Select and enter exploration mode.
		for i := 0; i < 3; i++ {
			if err := stadiacuj.PressKeyInGame(ctx, kb, "Right", time.Second); err != nil {
				s.Fatal("Failed to select Exploration Mode: ", err)
			}
		}
		if err := stadiacuj.PressKeyInGame(ctx, kb, "Enter", 20*time.Second); err != nil {
			s.Fatal("Failed to enter Exploration Mode: ", err)
		}
		gameOngoing = true
		// Game starts. Control the main character to move forward for 30 seconds.
		if err := stadiacuj.HoldKeyInGame(ctx, kb, "W", 30*time.Second); err != nil {
			s.Fatal("Failed to move forward: ", err)
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	// Check if there is any tab crashed.
	if err := tabChecker.Check(closeCtx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(closeCtx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the perf data: ", err)
	}
}
