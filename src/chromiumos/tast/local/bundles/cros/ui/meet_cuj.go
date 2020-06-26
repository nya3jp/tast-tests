// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/bond"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetCUJ,
		Desc:         "Measures the performance of critical user journey for Google Meet",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      3 * time.Minute,
		Pre:          cuj.LoggedInToCUJUser(),
		Vars: []string{
			"mute",
			"ui.MeetCUJ.bond_credentials",
			"ui.cuj_username",
			"ui.cuj_password",
		},
	})
}

func MeetCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout     = 10 * time.Second
		numBots     = 4
		botDuration = 2 * time.Minute
	)

	creds := s.RequiredVar("ui.MeetCUJ.bond_credentials")
	bc, err := bond.NewClient(ctx, bond.WithCredsJSON([]byte(creds)))
	if err != nil {
		s.Fatal("Failed to create a bond client: ", err)
	}
	defer bc.Close()

	var meetingCode string
	func() {
		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		meetingCode, err = bc.CreateConference(sctx)
		if err != nil {
			s.Fatal("Failed to create a conference room: ", err)
		}
	}()
	s.Log("Created a room with the code ", meetingCode)

	cr := s.PreValue().(cuj.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	conn, err := cr.NewConn(ctx, "https://meet.google.com/")
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer conn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Make it into a normal window if it is in clamshell-mode; so that the
	// desktop needs to compose the browser window with the wallpaper.
	inTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to detect it is in tablet-mode or not: ", err)
	}
	if !inTabletMode {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain the window list: ", err)
		}
		for _, w := range ws {
			if _, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventNormal); err != nil {
				s.Fatalf("Failed to change the window %s (%d) to normal: %v", w.Name, w.ID, err)
			}
		}
	}

	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)

	enter, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Enter meeting code"}, timeout)
	if err != nil {
		s.Fatal("Failed to find the meeting code: ", err)
	}
	defer enter.Release(ctx)
	if err := enter.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the input form: ", err)
	}
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	if err := kw.Type(ctx, meetingCode); err != nil {
		s.Fatal("Failed to type the meeting code: ", err)
	}
	if err := kw.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to type the enter key: ", err)
	}

	if err := func() error {
		// Meet will ask the permission: wait for the permission bubble to appear.
		// Note that there may be some other bubbles, so find only within the main
		// container -- which should be named as "Desk_Container_A", the primary
		// desk.
		container, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: "Desk_Container_A"})
		if err != nil {
			return errors.Wrap(err, "failed to find the container")
		}
		defer container.Release(ctx)
		for i := 0; i < 5; i++ {
			bubble, err := container.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "BubbleDialogDelegateView"}, timeout)
			if err != nil {
				// It is fine not finding the bubble.
				return nil
			}
			defer bubble.Release(ctx)
			allowButton, err := bubble.Descendant(ctx, ui.FindParams{Name: "Allow", Role: ui.RoleTypeButton})
			if err != nil {
				return errors.Wrap(err, "failed to find the allow button")
			}
			defer allowButton.Release(ctx)
			if err := allowButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click the allow button")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for the next cycle of permission")
			}
		}
		return errors.New("too many permission requests")
	}(); err != nil {
		s.Fatal("Failed to skip the permission requests: ", err)
	}

	askToJoin, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Ask to join"}, timeout)
	if err != nil {
		s.Fatal("Failed to find join-now button: ", err)
	}
	defer askToJoin.Release(ctx)

	configs := []cuj.MetricConfig{cuj.NewCustomMetricConfig(
		"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
		"percent", perf.SmallerIsBetter, []int64{50, 80})}
	for _, suffix := range []string{"Capturer", "Encoder", "EncoderQueue", "RateLimiter"} {
		configs = append(configs, cuj.NewCustomMetricConfig(
			"WebRTC.Video.DroppedFrames."+suffix, "percent", perf.SmallerIsBetter,
			[]int64{50, 80}))
	}
	recorder, err := cuj.NewRecorder(ctx, configs...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	pv := perf.NewValues()
	if err := recorder.Run(ctx, tconn, func() error {
		if err := askToJoin.LeftClick(ctx); err != nil {
			return errors.Wrap(err, `failed to click "Join now" button`)
		}
		shareMessage := "Share this info with people you want in the meeting"
		if err := webview.WaitUntilDescendantExists(ctx, ui.FindParams{Name: shareMessage}, timeout); err == nil {
			// "Share this code" popup appears, dismissing by close button.
			// Close button
			closeButton, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: "Close"}, timeout)
			if err != nil {
				return errors.Wrap(err, "close button should be in the popup")
			}
			if err := closeButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click the close button")
			}
			if err := webview.WaitUntilDescendantGone(ctx, ui.FindParams{Name: shareMessage}, timeout); err != nil {
				return errors.Wrap(err, "popup does not disappear")
			}
		}

		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if _, err := bc.AddBots(sctx, meetingCode, numBots, botDuration); err != nil {
			return errors.Wrap(err, "failed to create bots")
		}

		errc := make(chan error)
		go func() {
			// Using goroutine to measure GPU counters asynchronously because:
			// - we will add some test scenarios (controlling windows / meet sessions)
			//   rather than just sleeping.
			// - graphics.MeasureGPUCounters may quit immediately when the hardware or
			//   kernel does not support the reporting mechanism.
			errc <- graphics.MeasureGPUCounters(ctx, 30*time.Second, pv)
		}()
		s.Log("Waiting for 30 seconds")
		if err := testing.Sleep(ctx, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		// Close the browser window to finish it.
		if err := conn.CloseTarget(ctx); err != nil {
			return errors.Wrap(err, "failed to close")
		}
		if err := <-errc; err != nil {
			return errors.Wrap(err, "failed to collect GPU counters")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	// Before recording the metrics, check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}

	if err := recorder.Record(pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
