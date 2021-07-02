// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package everydaymultitaskingcuj contains the test code for Everyday MultiTasking CUJ.
package everydaymultitaskingcuj

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/cuj/volume"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

const (
	// YoutubeMusicAppName indicates to test against YoutubeMusic.
	YoutubeMusicAppName = "ytmusic"
	// SpotifyAppName indicates to test against Spotify.
	SpotifyAppName = "Spotify"

	initialVolume = 60
)

// RunParams holds the parameters to run the test main logic.
type RunParams struct {
	tier           cuj.Tier
	ccaScriptPaths []string // ccaSriptPaths is the scirpt paths used by CCA package to do camera testing.
	outDir         string
	appName        string
	account        string // account is the one used by Spotify APP to do login.
	tabletMode     bool
}

// NewRunParams constructs a RunParams struct and returns the pointer to it.
func NewRunParams(tier cuj.Tier, ccaScriptPaths []string, outDir, appName, account string, tabletMode bool) *RunParams {
	return &RunParams{tier: tier, ccaScriptPaths: ccaScriptPaths, outDir: outDir, appName: appName, account: account, tabletMode: tabletMode}
}

type runResources struct {
	kb        *input.KeyboardEventWriter
	topRow    *input.TopRowLayout
	ui        *uiauto.Context
	vh        *volume.Helper
	uiHandler cuj.UIActionHandler
	recorder  *cuj.Recorder
}

// Run runs the EverydayMultitaskingCUJ test.
func Run(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, params *RunParams) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

	var appSpotify *Spotify

	// Prepare ARC and install android apps for the everyday multitasking: Spotify.
	if params.appName == SpotifyAppName {
		if appSpotify, err = NewSpotify(ctx, kb, a, tconn, params.account); err != nil {
			return errors.Wrap(err, "failed to create Spotify instance")
		}
		defer func(ctx context.Context) {
			appSpotify.Close(ctx, cr, retErr != nil, filepath.Join(params.outDir, "arc"))
		}(cleanupCtx)

		testing.ContextLog(ctx, "Check and install ", spotifyPackageName)
		if err := appSpotify.Install(ctx); err != nil {
			return errors.Wrap(err, "failed to install Spotify")
		}
	}

	vh, err := volume.NewVolumeHelper(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the volumeHelper")
	}
	originalVolume, err := vh.GetVolume(ctx)
	defer vh.SetVolume(cleanupCtx, originalVolume)

	// uiHandler will be assigned with different instances for clamshell and tablet mode.
	var uiHandler cuj.UIActionHandler
	if params.tabletMode {
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create tablet action handler")
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create clamshell action handler")
		}
	}
	defer uiHandler.Close()

	testing.ContextLog(ctx, "Start to get browser start time")
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, params.tabletMode)
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
	defer recorder.Close(cleanupCtx)

	// Put battery under discharge in order to collect the power consumption of the test.
	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	// It's important to ensure setBatteryNormal will be called after the test is done.
	// So make it the first deferred function to use cleanupCtx.
	defer setBatteryNormal(cleanupCtx)

	faillogCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		// The screenshot and ui tree dump must been taken before tabs are closed.
		faillog.SaveScreenshotOnError(ctx, cr, params.outDir, func() bool { return retErr != nil })
		faillog.DumpUITreeOnError(ctx, params.outDir, func() bool { return retErr != nil }, tconn)
		cuj.CloseBrowserTabs(ctx, tconn)
	}(faillogCtx)

	var appStartTime int64
	// Launch arc apps from the app launcher; first open the app-launcher, type
	// the query and select the first search result, and wait for the app window
	// to appear. When the app has the splash screen, skip it.
	if params.appName == SpotifyAppName {
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			t, err := appSpotify.Launch(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to Launch Spotify")
			}
			appStartTime = t.Milliseconds()

			testing.ContextLog(ctx, "Start to play Spotify")
			if err = appSpotify.Play(ctx); err != nil {
				return errors.Wrap(err, "failed to play Spotify")
			}
			// Let spotify continue to play for some time.
			return testing.Sleep(ctx, 3*time.Second)
		}); err != nil {
			return errors.Wrap(err, "failed to launch Spotify")
		}
	}

	resources := &runResources{kb: kb, topRow: topRow, ui: ui, vh: vh, uiHandler: uiHandler, recorder: recorder}

	if err := openAndSwitchTabs(ctx, cr, tconn, params, resources); err != nil {
		return errors.Wrap(err, "failed to open and switch chrome tabs")
	}

	if err := switchWindows(ctx, cr, tconn, params, resources); err != nil {
		return errors.Wrap(err, "failed to switch windows")
	}

	testing.ContextLog(ctx, "Take photo and video")
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		return takePhotoAndVideo(ctx, cr, params.ccaScriptPaths, params.outDir)
	}); err != nil {
		return errors.Wrap(err, "failed to run the camera scenario")
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
		return errors.Wrap(err, "failed to report")
	}
	if err = pv.Save(params.outDir); err != nil {
		return errors.Wrap(err, "failed to store values")
	}
	if err := recorder.SaveHistograms(params.outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}

	return nil
}

func openAndSwitchTabs(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, params *RunParams, resources *runResources) error {
	const (
		gmailURL        = "https://mail.google.com"
		calendarURL     = "https://calendar.google.com/"
		youtubeMusicURL = "https://music.youtube.com/channel/UCPC0L1d253x-KuMNwa05TpA"
		huluURL         = "https://www.hulu.com/"
		googleNewsURL   = "https://news.google.com/"
		cnnNewsURL      = "https://edition.cnn.com/"
		wikiURL         = "https://www.wikipedia.org/"
		redditURL       = "https://www.reddit.com/"
	)

	// Basic tier test scenario: Have 2 browser windows open with 5 tabs each.
	// 1. The first window URL list including Gmail, Calendar, YouTube Music, Hulu and Google News.
	// 2. The second window URL list including Google News, CCN news, Wiki.
	// Plus tier test scenario: Same as basic but click through 20 tabs (4 windows x 5 tabs).
	// 1. The first and second window URL list are same as basic.
	// 2. The third window URL list including Google News, CNN news, Wikipedia, Reddit.
	// 3. The fourth window URL list is same as the third one.
	firstWindowURLList := []string{gmailURL, calendarURL, youtubeMusicURL, huluURL, googleNewsURL}
	secondWindowURLList := []string{googleNewsURL, cnnNewsURL, wikiURL, googleNewsURL, cnnNewsURL}
	thirdWindowURLList := []string{googleNewsURL, cnnNewsURL, wikiURL, redditURL, cnnNewsURL}
	fourthWindowURLList := thirdWindowURLList

	// Basic tier URL list that will be opened in two browser windows.
	pageList := [][]string{firstWindowURLList, secondWindowURLList}
	// Plus tier URL list that will be opened in four browser windows.
	if params.tier == cuj.Plus {
		pageList = append(pageList, thirdWindowURLList, fourthWindowURLList)
	}

	openBrowserWithTabs := func(urlList []string) error {
		for idx, url := range urlList {
			conn, err := resources.uiHandler.NewChromeTab(ctx, cr, url, idx == 0)
			if err != nil {
				return errors.Wrapf(err, "failed to open %s", url)
			}
			// We don't need to keep the connection, so close it before leaving this function.
			defer conn.Close()

			if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
				return errors.Wrap(err, "failed to wait for page to finish loading")
			}

			if params.appName == YoutubeMusicAppName && url == youtubeMusicURL {
				shuffleButton := nodewith.Name("Shuffle").Role(role.Button)
				pauseButton := nodewith.Name("Pause").Role(role.Button)

				if err := testing.Poll(ctx, func(ctx context.Context) error {
					return uiauto.Combine("play youtube music", resources.uiHandler.Click(shuffleButton), resources.ui.WaitUntilExists(pauseButton))(ctx)
				}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// switchTabsAndChangeVolume changes the volume after switching tabs.
	switchTabsAndChangeVolume := func(ctx context.Context, pages []string) error {
		if err := resources.vh.SetVolume(ctx, initialVolume); err != nil {
			return errors.Wrapf(err, "failed to set volume to %d percent", initialVolume)
		}

		for tabIdx := range pages {
			testing.ContextLog(ctx, "Switching Chrome tab")
			if err := resources.uiHandler.SwitchToChromeTabByIndex(tabIdx)(ctx); err != nil {
				return errors.Wrap(err, "failed to switch tab")
			}
			testing.ContextLog(ctx, "Volume up")
			if err := resources.vh.VerifyVolumeChanged(ctx, func() error {
				return resources.kb.Accel(ctx, resources.topRow.VolumeUp)
			}); err != nil {
				return errors.Wrap(err, `volume not changed after press "VolumeUp"`)
			}

			// After applying new volume, stay on the tab with the volume for 2 seconds before applying next one.
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
		}
		return nil
	}

	switchAllBrowserTabs := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start to switch all browser tabs")
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain the window list")
		}

		browserWinIdx := -1
		for range ws {
			// Cycle through all windows, find out the browser window, and do tab switch.

			// Switch to the least recent window to start from the window we opened first.
			testing.ContextLog(ctx, "Switching window by overview")
			if err := resources.uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughOverview)(ctx); err != nil {
				return errors.Wrap(err, "failed to switch windows through overview")
			}

			w, err := ash.GetActiveWindow(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get active window")
			}

			if w.WindowType != ash.WindowTypeBrowser {
				continue
			}
			browserWinIdx++
			if err := switchTabsAndChangeVolume(ctx, pageList[browserWinIdx]); err != nil {
				return errors.Wrap(err, "failed to switch tabs")
			}
		}
		return nil
	}

	if err := resources.recorder.Run(ctx, func(ctx context.Context) error {
		for _, list := range pageList {
			if err := openBrowserWithTabs(list); err != nil {
				return errors.Wrap(err, "failed to open browser with tabs")
			}
		}
		if err := switchAllBrowserTabs(ctx); err != nil {
			return errors.Wrap(err, "failed to switch all browser tabs")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to run the open tabs and switch tabs scenario")
	}
	return nil
}

func switchWindows(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, params *RunParams, resources *runResources) error {
	// subtest defines the detail of the window switch test procedure. It could be different for clamshell and tablet mode.
	type subtest struct {
		name string
		desc string
		// switchWindowFunc is the function used to do window switch.
		// ws is all the application windows in the system.
		// i is the index of the target window switching to.
		switchWindowFunc func(ctx context.Context, ws []*ash.Window, i int) error
	}
	// switchWindowByOverviewTest is the common switch window test for clamshell and tablet.
	switchWindowByOverviewTest := subtest{
		"overview",
		"Switching the focused window through the overview mode",
		func(ctx context.Context, ws []*ash.Window, i int) error {
			testing.ContextLog(ctx, "Switching window by overview")
			return resources.uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughOverview)(ctx)
		},
	}
	// switchWindowTests holds a serial of window switch tests. It has different subtest for clamshell and tablet mode.
	switchWindowTests := []subtest{switchWindowByOverviewTest}
	if params.tabletMode {
		switchWindowByHotseatTest := subtest{
			"hotseat",
			"Switching the focused window through clicking the hotseat",
			func(ctx context.Context, ws []*ash.Window, i int) error {
				wIdx := 0 // The index of target window within same app.
				for idx := 0; idx < i; idx++ {
					// Count the windows of same type before the target window.
					if ws[idx].WindowType == ws[i].WindowType {
						wIdx++
					}
				}
				winName := "Chrome"
				if strings.Contains(ws[i].Title, SpotifyAppName) {
					winName = SpotifyAppName
				}
				testing.ContextLogf(ctx, "Switching window to %q", ws[i].Title)
				return resources.uiHandler.SwitchToAppWindowByIndex(winName, wIdx)(ctx)
			},
		}
		switchWindowTests = append(switchWindowTests, switchWindowByHotseatTest)
	} else {
		switchWindowByKeyboardTest := subtest{
			"alt-tab",
			"Switching the focused window through Alt-Tab",
			func(ctx context.Context, ws []*ash.Window, i int) error {
				return resources.uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughKeyEvent)(ctx)
			},
		}
		switchWindowTests = append(switchWindowTests, switchWindowByKeyboardTest)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get window list")
	}

	for _, subtest := range switchWindowTests {
		testing.ContextLog(ctx, subtest.desc)
		if err := resources.recorder.Run(ctx, func(ctx context.Context) error {
			if err := resources.vh.SetVolume(ctx, initialVolume); err != nil {
				return errors.Wrapf(err, "failed to set volume to %v percents", initialVolume)
			}
			testing.ContextLog(ctx, "Volume up")
			if err := resources.vh.VerifyVolumeChanged(ctx, func() error {
				return resources.kb.Accel(ctx, resources.topRow.VolumeUp)
			}); err != nil {
				return errors.Wrap(err, `volume not changed after press "VolumeUp"`)
			}

			for i := range ws {
				// Switch between windows by calling the switch window function.
				if err := subtest.switchWindowFunc(ctx, ws, i); err != nil {
					return errors.Wrap(err, "failed to switch window")
				}
			}
			return nil
		}); err != nil {
			return errors.Wrap(err, "failed to run the switch window scenario")
		}
	}

	return nil
}

func takePhotoAndVideo(ctx context.Context, cr *chrome.Chrome, scriptPaths []string, outDir string) error {
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to clear saved directory")
	}

	app, err := cca.New(ctx, cr, scriptPaths, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	_, err = app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		return errors.Wrap(err, "failed to take single photo")
	}

	testing.ContextLog(ctx, "Switch to video mode")
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to video mode")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after switch to video mode")
	}

	testing.ContextLog(ctx, "Click shutter to start video recording")
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "failed to click shutter")
	}

	// Keep video recording for some time.
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	testing.ContextLog(ctx, "Click shutter to stop video recording")
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "failed to click shutter")
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "shutter is not ended")
	}

	return nil
}
