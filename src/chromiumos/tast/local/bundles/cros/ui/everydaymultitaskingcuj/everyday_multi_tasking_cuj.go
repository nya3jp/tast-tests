// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package everydaymultitaskingcuj contains the test code for Everyday MultiTasking CUJ.
package everydaymultitaskingcuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

const (
	// YoutubeMusic indicates to test against YoutubeMusic.
	YoutubeMusic = "ytmusic"
)

// Run runs the EverydayMultitaskingCUJ test.
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, tier cuj.Tier, appName string, tabletMode, enableBT bool) {
	const (
		gmailURL        = "https://mail.google.com"
		calendarURL     = "https://calendar.google.com/"
		youtubeMusicURL = "https://music.youtube.com/channel/UCPC0L1d253x-KuMNwa05TpA"
		huluURL         = "https://www.hulu.com/"
		googleNewsURL   = "https://news.google.com/"
		cnnNewsURL      = "https://edition.cnn.com/"
		wikiURL         = "https://www.wikipedia.org/"
		redditURL       = "https://www.reddit.com/"
		initialVolume   = 60
		intervalVolume  = 5
		timeout         = 3 * time.Second
	)

	firstURLList := []string{
		gmailURL,
		calendarURL,
		youtubeMusicURL,
		huluURL,
		googleNewsURL,
	}
	basicURLList := []string{
		googleNewsURL,
		cnnNewsURL,
		wikiURL,
		googleNewsURL,
		cnnNewsURL,
	}
	plusURLList := []string{
		googleNewsURL,
		cnnNewsURL,
		wikiURL,
		redditURL,
		cnnNewsURL,
	}

	pageList := [][]string{
		firstURLList,
		basicURLList,
	}

	if tier == cuj.Plus {
		pageList = append(pageList, plusURLList, plusURLList)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	ui := uiauto.New(tconn)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	var uiHandler cuj.UIActionHandler
	var pc pointer.Context
	type subtest struct {
		name string
		desc string
		f    func(ctx context.Context, s *testing.State, i int) error
	}
	browserWindows := map[int]bool{}
	var ws []*ash.Window
	var subtest2 subtest

	if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("Failed create chrome action handler: ", err)
	}
	pc = pointer.NewMouse(tconn)
	subtest2 = subtest{
		"alt-tab",
		"Switching the focused window through Alt-Tab",
		func(ctx context.Context, s *testing.State, i int) error {
			return uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughKeyEvent)(ctx)
		},
	}

	defer pc.Close()
	defer uiHandler.Close()

	s.Log("Start to get browser start time")
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}

	// Set up the cuj.Recorder: this test will measure the combinations of
	// animation smoothness for window-cycles (alt-tab selection), launcher,
	// and overview.
	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	var appStartTime int64
	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	} else {
		defer setBatteryNormal(ctx)
	}

	openBrowserWithTabs := func(urlList []string) error {
		var conn *chrome.Conn
		for idx, url := range urlList {
			conn, err = uiHandler.NewChromeTab(ctx, cr, url, idx == 0)
			if err != nil {
				return errors.Wrapf(err, "failed to open %s", url)
			}
			if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
				return errors.Wrap(err, "failed to wait for page to finish loading")
			}
			// We don't need to keep the connection, so close it when ever leave this function.
			defer conn.Close()

			if appName == YoutubeMusic && url == youtubeMusicURL {
				shuffleButton := nodewith.Name("Shuffle").Role(role.Button)
				pauseButton := nodewith.Name("Pause").Role(role.Button)

				if err := testing.Poll(ctx, func(ctx context.Context) error {
					return uiauto.Combine("play youtube music", pc.Click(shuffleButton), ui.WaitUntilExists(pauseButton))(ctx)
				}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
					return err
				}
			}
		}

		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			if w.WindowType != ash.WindowTypeBrowser {
				return false
			}
			return !browserWindows[w.ID]
		})
		if err != nil {
			return errors.Wrapf(err, "failed to find the browser window for %s", urlList[0])
		}
		browserWindows[w.ID] = true
		if !tabletMode {
			if _, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventNormal); err != nil {
				return errors.Wrapf(err, "failed to change the window (%s) into the normal state", urlList[0])
			}
		}
		return nil
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		for _, list := range pageList {
			if err := openBrowserWithTabs(list); err != nil {
				return errors.Wrap(err, "failed to open browser with tabs")
			}
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to run the open tabs and switch tabs scenario: ", err)
	}

	subtests := []subtest{
		{
			"overview",
			"Switching the focused window through the overview mode",
			func(ctx context.Context, s *testing.State, i int) error {
				s.Log("Switching window by overview")
				return uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughOverview)(ctx)
			},
		},
		subtest2,
	}

	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get window list: ", err)
	}

	for _, st := range subtests {
		s.Log(st.desc)
		s.Run(ctx, st.name, func(ctx context.Context, s *testing.State) {
			if err := recorder.Run(ctx, func(ctx context.Context) error {
				for i := 0; i < len(ws); i++ {
					s.Log("Volume up")
					if err := kb.Accel(ctx, topRow.VolumeUp); err != nil {
						return errors.Wrap(err, "failed to turn volume up")
					}

					if err := st.f(ctx, s, i); err != nil {
						return errors.Wrap(err, "failed to switch window")
					}
				}
				return nil
			}); err != nil {
				s.Fatal("Failed to run the switch window scenario: ", err)
			}
		})
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))
	if appStartTime > 0 {
		pv.Set(perf.Metric{
			Name:      "Apps.StartTime",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(appStartTime))
	}
	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}

func waitAndClickObject(ctx context.Context, object *androidui.Object, name string, timeout time.Duration) error {
	if err := object.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrapf(err, `failed to find %q`, name)
	}
	if err := object.Click(ctx); err != nil {
		return errors.Wrapf(err, `failed to click %q`, name)
	}
	return nil
}
