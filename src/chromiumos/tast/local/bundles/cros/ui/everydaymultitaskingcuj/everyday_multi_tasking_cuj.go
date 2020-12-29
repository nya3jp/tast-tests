// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package everydaymultitaskingcuj contains the test code for Everyday MultiTasking CUJ.
package everydaymultitaskingcuj

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// YoutubeMusic indicates to test against YoutubeMusic.
const YoutubeMusic = "ytmusic"

// Run runs the EverydayMultitaskingCUJ test.
// ccaSriptPaths is the scirpt paths used by CCA package to do camera testing.
// account is the one used by Spotify APP to do login.
func Run(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, tier cuj.Tier, ccaScriptPaths []string, outDir, appName, account string, tabletMode bool) error {
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

	// Basic tier test scenario: Have 2 browser windows open with 5 tabs each.
	// 1. The first window URL list including Gmail, Calendar, YouTube Music, Hulu and Google News.
	// 2. The second window URL list including Google News, CCN news, Wiki.
	firstWindowURLList := []string{gmailURL, calendarURL, youtubeMusicURL, huluURL, googleNewsURL}
	secondWindowURLList := []string{googleNewsURL, cnnNewsURL, wikiURL, googleNewsURL, cnnNewsURL}

	// Basic tier URL list that will be opened in two browser windows.
	pageList := [][]string{firstWindowURLList, secondWindowURLList}
	// TODO: add plus tier URLs.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}
	ui := uiauto.New(tconn)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up ARC and Play Store")
	}
	defer d.Close(ctx)

	// uiHandler will be assigned with different instances for clamshell and tablet mode.
	var uiHandler cuj.UIActionHandler
	// subtest defines the detail of the window switch test procedure. It could be different for clamshell and tablet mode.
	type subtest struct {
		name string
		desc string
		// switchWindow is the function used to do window switch.
		// ws is all the applicaiton windows in the system.
		// i is the index of the target window switching to.
		switchWindow func(ctx context.Context, ws []*ash.Window, i int) error
	}

	// switchWindowByOverview is the common switch window test for clamshell and tablet.
	switchWindowByOverview := subtest{
		"overview",
		"Switching the focused window through the overview mode",
		func(ctx context.Context, ws []*ash.Window, i int) error {
			testing.ContextLog(ctx, "Switching window by overview")
			return uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughOverview)(ctx)
		},
	}
	// switchWindowTest holds a serial of window switch tests. It has different subtest for clamshell and tablet mode.
	var switchWindowTest []subtest
	if tabletMode {
		// TODO: Add tablet mode.
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create chrome action handler")
		}
		defer uiHandler.Close()

		switchWindowByKeyboard := subtest{
			"alt-tab",
			"Switching the focused window through Alt-Tab",
			func(ctx context.Context, ws []*ash.Window, i int) error {
				return uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughKeyEvent)(ctx)
			},
		}
		switchWindowTest = []subtest{
			switchWindowByOverview,
			switchWindowByKeyboard,
		}
	}

	testing.ContextLog(ctx, "Start to get browser start time")
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

	// Set up the cuj.Recorder: this test will measure the combinations of
	// animation smoothness for window-cycles (alt-tab selection), launcher,
	// and overview.
	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(ctx)

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	defer setBatteryNormal(ctx)
	// Shorten the context to make sure battery charging is resumed after testing.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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
			// We don't need to keep the connection, so close it before leaving this function.
			defer conn.Close()

			if appName == YoutubeMusic && url == youtubeMusicURL {
				shuffleButton := nodewith.Name("Shuffle").Role(role.Button)
				pauseButton := nodewith.Name("Pause").Role(role.Button)

				if err := testing.Poll(ctx, func(ctx context.Context) error {
					return uiauto.Combine("play youtube music", uiHandler.Click(shuffleButton), ui.WaitUntilExists(pauseButton))(ctx)
				}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
					return err
				}
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
		// TODO: do switch browser tabs.
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to run the open tabs and switch tabs scenario")
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get window list")
	}

	for _, subtest := range switchWindowTest {
		testing.ContextLog(ctx, subtest.desc)
		if err := recorder.Run(ctx, func(ctx context.Context) error {
			// TODO: do volume change.

			for i := 0; i < len(ws); i++ {
				testing.ContextLog(ctx, "Volume up")
				if err := kb.Accel(ctx, topRow.VolumeUp); err != nil {
					return errors.Wrap(err, "failed to turn volume up")
				}
				// Switch between windows by calling the switch window function.
				if err := subtest.switchWindow(ctx, ws, i); err != nil {
					return errors.Wrap(err, "failed to switch window")
				}
			}
			return nil
		}); err != nil {
			return errors.Wrap(err, "failed to run the switch window scenario")
		}
	}
	// TODO: take photo and video.
	// TODO: Save recorder metrics.
	return nil
}
