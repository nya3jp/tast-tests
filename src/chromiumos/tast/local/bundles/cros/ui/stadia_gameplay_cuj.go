// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/stadiacuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StadiaGameplayCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for game playing on Stadia",
		Contacts:     []string{"yichenz@chromium.org"},
		// TODO(http://crbug/1144356): Test is disabled until it can be fixed
		// Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Vars:         []string{"record"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// StadiaGameplayCUJ test starts and plays a exploration scene and gathering the performance.
// The game playing is hardcoded.
func StadiaGameplayCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout = 30 * time.Second
	)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create ScreenRecorder: ", err)
		}
		defer func() {
			screenRecorder.Stop(ctx)
			dir, ok := testing.ContextOutDir(ctx)
			if ok && dir != "" {
				if _, err := os.Stat(dir); err == nil {
					testing.ContextLogf(ctx, "Saving screen record to %s", dir)
					if err := screenRecorder.SaveInBytes(ctx, filepath.Join(dir, "screen_record.webm")); err != nil {
						s.Fatal("Failed to save screen record in bytes: ", err)
					}
				}
			}
			screenRecorder.Release(ctx)
		}()
		screenRecorder.Start(ctx, tconn)
	}

	// Set up the browser.
	bt := s.Param().(browser.Type)
	conn, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, bt, stadiacuj.StadiaGameURL)
	if err != nil {
		s.Fatal("Failed to open the stadia staging instance: ", err)
	}
	defer closeBrowser(closeCtx)
	defer conn.Close()

	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get TestAPIConn: ", err)
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	// Ash metrics config, always collected from ash-chrome.
	ashConfigs := []cujrecorder.MetricConfig{
		cujrecorder.NewCustomMetricConfig(
			"Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent",
			perf.SmallerIsBetter),
		cujrecorder.NewCustomMetricConfig(
			"Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks",
			perf.SmallerIsBetter),
	}
	if err := recorder.AddCollectedMetrics(tconn, browser.TypeAsh, ashConfigs...); err != nil {
		s.Fatal("Failed to add Ash recorded metrics: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	// Browser metrics config, collected from ash-chrome or lacros-chrome
	// depending on the browser being used.
	browserConfigs := []cujrecorder.MetricConfig{
		cujrecorder.NewCustomMetricConfig(
			"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent",
			perf.SmallerIsBetter),
	}
	if err := recorder.AddCollectedMetrics(bTconn, bt, browserConfigs...); err != nil {
		s.Fatal("Failed to add Browser recorded metrics: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn).WithTimeout(timeout)
	webview := nodewith.ClassName("ContentsWebView").Role(role.WebView)

	// Launch the game.
	gameLaunchButton := nodewith.Role(role.Button).Name("Play").Ancestor(webview)
	if err := uiauto.Combine(
		"wait and make visible",
		ac.WaitUntilExists(gameLaunchButton),
		ac.MakeVisible(gameLaunchButton))(ctx); err != nil {
		s.Fatal("Failed to make the game launch button visible: ", err)
	}
	gameStartButton := nodewith.Name(stadiacuj.StadiaGameName + " Play game").Role(role.Button).Ancestor(webview)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ac.Exists(gameStartButton)(ctx); err == nil {
			return nil
		}
		if err := ac.LeftClick(gameLaunchButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the game launch button")
		}
		return errors.New("game hasn't launched yet")
	}, &testing.PollOptions{Interval: time.Second, Timeout: timeout}); err != nil {
		s.Fatal("Failed to launch the game: ", err)
	}

	// Make sure the game is fully launched.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	id0 := ws[0].ID
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		w0, err := ash.GetWindow(ctx, tconn, id0)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the window"))
		}
		// If the window is already in full screen, the game has already started and
		// no need to press the start button.
		if w0.State == ash.WindowStateFullscreen {
			return nil
		}
		if err = kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "failed to click the game start button")
		}
		return errors.New("game hasn't started yet")
	}, &testing.PollOptions{Interval: time.Second, Timeout: timeout}); err != nil {
		s.Fatal("Failed to start the game: ", err)
	}

	// If internet is unstable, try to start again one time.
	if err := ac.WithTimeout(45 * time.Second).WaitUntilExists(nodewith.Name("Try again"))(ctx); err == nil {
		// Try again if the button exists.
		pollOpts := testing.PollOptions{Interval: time.Second, Timeout: timeout}
		if err := ac.WithPollOpts(pollOpts).LeftClick(nodewith.Name("Try again"))(ctx); err != nil {
			s.Fatal("Failed to click the try again button: ", err)
		}
	}

	// Wait for the game to be completely loaded and the opening animation to be done.
	if err := testing.Sleep(ctx, time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	gameOngoing := false
	defer func() {
		if gameOngoing {
			// Exit the game.
			if err := stadiacuj.ExitGame(closeCtx, kb, ac, webview); err != nil {
				s.Error("Failed to exit game: ", err)
			}
		}
	}()

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Hard code the game playing routine.
		// Enter the menu.
		if err := stadiacuj.PressKey(ctx, kb, "Enter", 10*time.Second); err != nil {
			s.Fatal("Failed to enter the menu: ", err)
		}
		// Enter story mode.
		if err := stadiacuj.PressKey(ctx, kb, "Enter", time.Second); err != nil {
			s.Fatal("Failed to enter Story Mode: ", err)
		}
		// Select and enter exploration mode.
		for i := 0; i < 3; i++ {
			if err := stadiacuj.PressKey(ctx, kb, "Right", time.Second); err != nil {
				s.Fatal("Failed to select Exploration Mode: ", err)
			}
		}
		if err := stadiacuj.PressKey(ctx, kb, "Enter", 20*time.Second); err != nil {
			s.Fatal("Failed to enter Exploration Mode: ", err)
		}
		gameOngoing = true
		// Game starts. Control the main character to move forward for 30 seconds.
		if err := stadiacuj.HoldKey(ctx, kb, "W", 30*time.Second); err != nil {
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
