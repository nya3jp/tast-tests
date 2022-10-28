// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/spotify"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

const (
	// HelloWorldAppName indicates to test against a "Hello world" ARC app.
	HelloWorldAppName = "Hello world"
	// YoutubeMusicAppName indicates to test against YoutubeMusic.
	YoutubeMusicAppName = "ytmusic"
	// SpotifyAppName indicates to test against Spotify.
	SpotifyAppName = spotify.AppName

	helloworldAPKName      = "ArcAppValidityTest.apk"
	helloworldPackageName  = "org.chromium.arc.testapp.appvaliditytast"
	helloworldActivityName = ".MainActivity"

	initialVolume   = 60
	mediumUITimeout = 30 * time.Second // Used for situations where UI response are slower.
)

// RunParams holds the parameters to run the test main logic.
type RunParams struct {
	tier            cuj.Tier
	ccaScriptPaths  []string // ccaSriptPaths is the scirpt paths used by CCA package to do camera testing.
	outDir          string
	appName         string
	account         string // account is the one used by Spotify APP to do login.
	tabletMode      bool
	enableBT        bool
	traceConfigPath string
}

// NewRunParams constructs a RunParams struct and returns the pointer to it.
func NewRunParams(tier cuj.Tier, ccaScriptPaths []string, outDir, appName, account, traceConfigPath string,
	tabletMode, enableBT bool) *RunParams {
	return &RunParams{tier: tier,
		ccaScriptPaths:  ccaScriptPaths,
		outDir:          outDir,
		appName:         appName,
		account:         account,
		traceConfigPath: traceConfigPath,
		tabletMode:      tabletMode,
		enableBT:        enableBT,
	}
}

type runResources struct {
	kb         *input.KeyboardEventWriter
	topRow     *input.TopRowLayout
	ui         *uiauto.Context
	vh         *audio.Helper
	uiHandler  cuj.UIActionHandler
	recorder   *cujrecorder.Recorder
	browserApp apps.App
}

// Run runs the EverydayMultitaskingCUJ test.
func Run(ctx context.Context, cr *chrome.Chrome, bt browser.Type, a *arc.ARC, params *RunParams) (retErr error) {
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

	var appHelloWorld *arc.Activity
	var appSpotify *spotify.Spotify
	switch params.appName {
	case HelloWorldAppName:
		testing.ContextLog(ctx, "Install ", helloworldPackageName)
		if err := a.Install(ctx, arc.APKPath(helloworldAPKName)); err != nil {
			return errors.Wrap(err, "failed to install \"Hello world\" ARC app")
		}

		if appHelloWorld, err = arc.NewActivity(a, helloworldPackageName, helloworldActivityName); err != nil {
			return errors.Wrap(err, "failed to create activity for \"Hello world\" ARC app")
		}
		defer appHelloWorld.Close()
	case SpotifyAppName:
		if appSpotify, err = spotify.New(ctx, kb, a, tconn, params.account); err != nil {
			return errors.Wrap(err, "failed to create Spotify instance")
		}
		defer appSpotify.Close(cleanupCtx, cr, func() bool { return retErr != nil }, filepath.Join(params.outDir, "arc"))

		if err := appSpotify.Install(ctx); err != nil {
			return errors.Wrap(err, "failed to install Spotify")
		}
	}

	vh, err := audio.NewVolumeHelper(ctx)
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
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, params.tabletMode, bt)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}
	if l != nil {
		defer l.Close(ctx)
	}
	br := cr.Browser()
	var bTconn *chrome.TestConn
	if l != nil {
		br = l.Browser()
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get lacros test API conn")
		}
	}
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not find the Chrome app")
	}

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to set initial settings")
	}
	defer cleanupSetting(cleanupSettingsCtx)

	faillogCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		// The screenshot and ui tree dump must been taken before tabs are closed.
		faillog.SaveScreenshotOnError(ctx, cr, params.outDir, func() bool { return retErr != nil })
		faillog.DumpUITreeOnError(ctx, params.outDir, func() bool { return retErr != nil }, tconn)
		cuj.CloseChrome(ctx, tconn)
	}(faillogCtx)

	// Set up the cuj.Recorder: this test will measure the combinations of
	// animation smoothness for window-cycles (alt-tab selection), launcher,
	// and overview.
	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	options := cujrecorder.NewPerformanceCUJOptions()
	options.DoNotChangeBluetooth = params.enableBT
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, a, options)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		return errors.Wrap(err, "failed to add metrics to recorder")
	}
	if params.traceConfigPath != "" {
		recorder.EnableTracing(params.outDir, params.traceConfigPath)
	}
	var appStartTime int64
	switch params.appName {
	case HelloWorldAppName:
		testing.ContextLog(ctx, "Launch \"Hello world\" ARC app")
		if err := recorder.Run(ctx, func(ctx context.Context) error {
			startTime := time.Now()
			// Use arc.WithWaitForLaunch() because we are measuring how long the launch takes.
			if err := appHelloWorld.Start(ctx, tconn, arc.WithWaitForLaunch()); err != nil {
				return err
			}
			appStartTime = time.Since(startTime).Milliseconds()
			return nil
		}); err != nil {
			return errors.Wrap(err, "failed to launch \"Hello world\" ARC app")
		}
	case SpotifyAppName:
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			t, err := appSpotify.Launch(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to Launch Spotify")
			}
			appStartTime = t.Milliseconds()

			testing.ContextLog(ctx, "Start to play Spotify")
			if err = appSpotify.Play(ctx, apputil.NewMedia("Photograph", "Song • Ed Sheeran")); err != nil {
				return errors.Wrap(err, "failed to play Spotify")
			}
			// Let spotify continue to play for some time.
			return testing.Sleep(ctx, 3*time.Second)
		}); err != nil {
			return errors.Wrap(err, "failed to launch Spotify")
		}
	}

	resources := &runResources{kb: kb, topRow: topRow, ui: ui, vh: vh, uiHandler: uiHandler, recorder: recorder, browserApp: browserApp}

	if err := openAndSwitchTabs(ctx, br, tconn, params, resources); err != nil {
		return errors.Wrap(err, "failed to open and switch chrome tabs")
	}

	if err := switchWindows(ctx, tconn, params, resources); err != nil {
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

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
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

func openAndSwitchTabs(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn, params *RunParams, resources *runResources) error {
	// Basic tier test scenario: Have 2 browser windows open with 5 tabs each.
	// 1. The first window URL list including Gmail, Calendar, YouTube Music, Hulu and Google News.
	// 2. The second window URL list including Google News, CCN news, Wiki.
	// Plus tier test scenario: Same as basic but click through 20 tabs (4 windows x 5 tabs).
	// 1. The first and second window URL list are same as basic.
	// 2. The third window URL list including Google News, CNN news, Wikipedia, Reddit.
	// 3. The fourth window URL list is same as the third one.
	firstWindowURLList := []string{cuj.GmailURL, cuj.GoogleCalendarURL, cuj.YoutubeMusicURL, cuj.HuluURL, cuj.GoogleNewsURL}
	secondWindowURLList := []string{cuj.GoogleNewsURL, cuj.CnnURL, cuj.WikipediaURL, cuj.GoogleNewsURL, cuj.CnnURL}
	thirdWindowURLList := []string{cuj.GoogleNewsURL, cuj.CnnURL, cuj.WikipediaURL, cuj.RedditURL, cuj.CnnURL}
	fourthWindowURLList := thirdWindowURLList

	// Basic tier URL list that will be opened in two browser windows.
	pageList := [][]string{firstWindowURLList, secondWindowURLList}
	// Plus tier URL list that will be opened in four browser windows.
	if params.tier == cuj.Plus {
		pageList = append(pageList, thirdWindowURLList, fourthWindowURLList)
	}

	openBrowserWithTabs := func(urlList []string) error {
		for idx, url := range urlList {
			conn, err := resources.uiHandler.NewChromeTab(ctx, br, url, idx == 0)
			if err != nil {
				return errors.Wrapf(err, "failed to open %s", url)
			}
			// We don't need to keep the connection, so close it before leaving this function.
			defer conn.Close()

			timeout := time.Minute
			if err := webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
				// It has been seen that content sites such as CNN (https://edition.cnn.com/) sometimes can take
				// minutes to reach quiescence on DUTs. When this occurred, it can be seen from screenshots that
				// the UI has actually loaded but background tasks prevented the site to reach quiescence. Therefore,
				// logic is added here to check whether the site has loaded. If the site has loaded, i.e., the site
				// readyState is not "loading", no error will be returned here.
				if err := conn.WaitForExpr(ctx, `document.readyState === "interactive" || document.readyState === "complete"`); err == nil {
					testing.ContextLogf(ctx, "%s could not reach quiescence, but document state has passed loading", url)
					continue
				}
				return errors.Wrapf(err, "failed to wait for page to finish loading within %v [%s]", timeout, url)
			}

			if params.appName == YoutubeMusicAppName && url == cuj.YoutubeMusicURL {
				if err := playYoutubeMusic(ctx, resources); err != nil {
					return errors.Wrap(err, "failed to play Youtube Music")
				}
			}
		}
		return nil
	}

	// switchTabsAndChangeVolume changes the volume after switching tabs.
	switchTabsAndChangeVolume := func(ctx context.Context, browserWinIdx int, pages []string) error {
		if err := resources.vh.SetVolume(ctx, initialVolume); err != nil {
			return errors.Wrapf(err, "failed to set volume to %d percent", initialVolume)
		}

		for tabIdx := range pages {
			testing.ContextLog(ctx, "Switching Chrome tab")
			if err := resources.uiHandler.SwitchToChromeTabByIndex(tabIdx)(ctx); err != nil {
				// Sometimes, Spotify may pop up ads as active window.
				// It should switch back to the browser window and try again.
				if err := uiauto.Combine("switch back to browser and switch tab again",
					resources.uiHandler.SwitchToAppWindowByIndex(resources.browserApp.Name, browserWinIdx),
					resources.uiHandler.SwitchToChromeTabByIndex(tabIdx),
				)(ctx); err != nil {
					return err
				}
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
			if w.WindowType != ash.WindowTypeBrowser && w.WindowType != ash.WindowTypeLacros {
				continue
			}
			browserWinIdx++
			if err := switchTabsAndChangeVolume(ctx, browserWinIdx, pageList[browserWinIdx]); err != nil {
				return errors.Wrap(err, "failed to switch tabs")
			}
		}
		return nil
	}

	if err := resources.recorder.Run(ctx, func(ctx context.Context) error {
		if resources.browserApp.ID == apps.LacrosID {
			activeWindow, err := ash.GetActiveWindow(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get the active window")
			}
			if activeWindow.WindowType != ash.WindowTypeLacros {
				if err := resources.uiHandler.SwitchToAppWindow(resources.browserApp.Name)(ctx); err != nil {
					return errors.Wrap(err, "failed to switch to lacros window")
				}
			}
		}
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

func switchWindows(ctx context.Context, tconn *chrome.TestConn, params *RunParams, resources *runResources) error {
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
				winName := resources.browserApp.Name
				switchFunc := resources.uiHandler.SwitchToAppWindowByIndex(winName, wIdx)
				for _, appName := range []string{HelloWorldAppName, SpotifyAppName} {
					if strings.Contains(ws[i].Title, appName) {
						winName = appName
						// Use SwitchToAppWindow() because the app has only one window.
						switchFunc = resources.uiHandler.SwitchToAppWindow(appName)
					}
				}
				testing.ContextLogf(ctx, "Switching window to %q", ws[i].Title)
				return switchFunc(ctx)
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
	tb, err := testutil.NewTestBridgeWithoutTestConfig(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
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

	testing.ContextLog(ctx, "Start to record video")
	if _, err := app.RecordVideo(ctx, cca.TimerOn, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to record video")
	}

	return nil
}

func playYoutubeMusic(ctx context.Context, resources *runResources) error {
	ui := resources.ui
	uiHdl := resources.uiHandler
	shuffleButton := nodewith.Name("Shuffle").Role(role.Button)
	pauseButton := nodewith.Name("Pause").Role(role.Button).First()
	reviewIconUpdateWindow := nodewith.Name("Review icon update").Role(role.Window)
	okButton := nodewith.Name("OK").Role(role.Button).Ancestor(reviewIconUpdateWindow)
	dismissReviewIconUpdateIfPresent := uiauto.IfSuccessThen(
		ui.WithTimeout(3*time.Second).WaitUntilExists(reviewIconUpdateWindow),
		uiauto.NamedAction("close 'Review icon update' dialog", uiHdl.Click(okButton)))
	waitPauseButton := uiauto.NamedCombine("wait pause button",
		dismissReviewIconUpdateIfPresent,
		ui.WaitUntilExists(pauseButton),
	)
	// Sometimes closing the dialog doesn't work, so add retry here.
	return uiauto.NamedAction("play youtube music",
		ui.WithTimeout(mediumUITimeout).DoDefaultUntil(shuffleButton, waitPauseButton))(ctx)
}
