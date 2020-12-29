// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package everydaymultitaskingcuj contains the test code for Everyday MultiTasking CUJ.
package everydaymultitaskingcuj

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
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
	// Spotify indicates to test against Spotify.
	Spotify = "Spotify"
)

// Run runs the EverydayMultitaskingCUJ test.
// ccaSriptPaths is the scirpt paths used by CCA package to do camera testing.
// account is the one used by Spotify APP to do login.
func Run(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, tier cuj.Tier, ccaScriptPaths []string, outDir, appName, account string, tabletMode bool) error {
	const (
		spotifyPackageName = "com.spotify.music"
		gmailURL           = "https://mail.google.com"
		calendarURL        = "https://calendar.google.com/"
		youtubeMusicURL    = "https://music.youtube.com/channel/UCPC0L1d253x-KuMNwa05TpA"
		huluURL            = "https://www.hulu.com/"
		googleNewsURL      = "https://news.google.com/"
		cnnNewsURL         = "https://edition.cnn.com/"
		wikiURL            = "https://www.wikipedia.org/"
		redditURL          = "https://www.reddit.com/"
		initialVolume      = 60
		intervalVolume     = 5
		timeout            = 3 * time.Second
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
	if tier == cuj.Plus {
		pageList = append(pageList, thirdWindowURLList, fourthWindowURLList)
	}

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

	// uiHandler and pc will be assign different instances for clamshell and tablet mode.
	var uiHandler cuj.UIActionHandler
	var pc pointer.Context
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
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create chrome action handler")
		}
		defer uiHandler.Close()

		if pc, err = pointer.NewTouch(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create touch context")
		}
		defer pc.Close()

		switchWindowByHotseat := subtest{
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
				appName = "Chrome"
				if strings.Contains(ws[i].Title, Spotify) {
					appName = Spotify
				}
				testing.ContextLogf(ctx, "Switching window to %q", ws[i].Title)
				return uiHandler.SwitchToAppWindowByIndex(appName, wIdx)(ctx)
			},
		}
		switchWindowTest = []subtest{
			switchWindowByOverview,
			switchWindowByHotseat,
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create chrome action handler")
		}
		defer uiHandler.Close()

		pc = pointer.NewMouse(tconn)
		defer pc.Close()
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

	// Install android apps for the everyday works: Spotify.
	if appName == Spotify {
		installSpotify := func(ctx context.Context) error {
			testing.ContextLog(ctx, "Check and install ", spotifyPackageName)
			installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()
			if err = playstore.InstallApp(installCtx, a, d, spotifyPackageName, -1); err != nil {
				return errors.Wrapf(err, "failed to install %s", spotifyPackageName)
			}
			if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
				return errors.Wrap(err, "failed to close Play Store")
			}
			return nil
		}
		if err := installSpotify(ctx); err != nil {
			return errors.Wrap(err, "failed to install Spotify")
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

	var appStartTime int64
	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	defer setBatteryNormal(ctx)
	// Shorten the context to make sure battery charging is resumed after testing.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Launch arc apps from the app launcher; first open the app-launcher, type
	// the query and select the first search result, and wait for the app window
	// to appear. When the app has the splash screen, skip it.

	if appName == Spotify {
		playSotify := func(ctx context.Context) error {
			const (
				spotifyIDPrefix   = "com.spotify.music:id/"
				playPauseButtonID = spotifyIDPrefix + "play_pause_button"
				searchTabID       = spotifyIDPrefix + "search_tab"
				searchFieldID     = spotifyIDPrefix + "find_search_field_text"
				queryID           = spotifyIDPrefix + "query"
				childrenID        = spotifyIDPrefix + "children"
				albumName         = "Photograph"
				singerName        = "Song • Ed Sheeran"
				defaultUITimeout  = 30 * time.Second
				waitTime          = 3 * time.Second
			)
			fisrtLogin := false
			signIn := d.Object(androidui.Text("Continue with Google"))
			if err := signIn.WaitForExists(ctx, waitTime); err != nil {
				testing.ContextLog(ctx, `Failed to find "Continue with Google" button`)
			} else if err := signIn.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "Continue with Google" button`)
			} else {
				accountButton := d.Object(androidui.Text(account))
				if err := waitAndClickObject(ctx, accountButton, "account button", waitTime); err != nil {
					testing.ContextLog(ctx, "Sign in directly")
				}
				fisrtLogin = true
			}

			dismiss := d.Object(androidui.Text("DISMISS"))
			if err := dismiss.WaitForExists(ctx, waitTime); err != nil {
				testing.ContextLog(ctx, `Failed to find "DISMISS" button, believing splash screen has been dismissed already`)
			} else if err := dismiss.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "DISMISS" button`)
			}

			promp := d.Object(androidui.Text("NO, THANKS"))
			if err := promp.WaitForExists(ctx, waitTime); err != nil {
				testing.ContextLog(ctx, `Failed to find "NO, THANKS" button`)
			} else if err := promp.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "NO, THANKS" button`)
			}

			testing.ContextLog(ctx, "Check if the Play button exists, click play button or search song to play")
			pauseButton := d.Object(androidui.ID(playPauseButtonID), androidui.Enabled(true), androidui.Description("Pause"))
			playButton := d.Object(androidui.ID(playPauseButtonID), androidui.Enabled(true), androidui.Description("Play"))

			if err := playButton.WaitForExists(ctx, timeout); err != nil {
				testing.ContextLog(ctx, "The play button doesn't exists")
			} else {
				if err := playButton.Click(ctx); err != nil {
					return errors.Wrap(err, `failed to click "play button"`)
				}
				if err := pauseButton.WaitForExists(ctx, defaultUITimeout); err != nil {
					testing.ContextLog(ctx, "The pause button doesn't exists")
				} else {
					return nil
				}
			}

			searchTab := d.Object(androidui.ID(searchTabID))
			if err := waitAndClickObject(ctx, searchTab, "search tab", defaultUITimeout); err != nil {
				return err
			}

			searchField := d.Object(androidui.ID(searchFieldID))
			if err := waitAndClickObject(ctx, searchField, "search feild", defaultUITimeout); err != nil {
				return err
			}

			query := d.Object(androidui.ID(queryID))
			if err := waitAndClickObject(ctx, query, "query feild", defaultUITimeout); err != nil {
				return err
			}

			if err := kb.Type(ctx, albumName); err != nil {
				return errors.Wrap(err, "failed to type album")
			}

			singerButton := d.Object(androidui.Text(singerName))
			if err := waitAndClickObject(ctx, singerButton, "singerButton", defaultUITimeout); err != nil {
				return err
			}

			var shufflePlayButton *androidui.Object
			if fisrtLogin {
				shufflePlayButton = d.Object(androidui.Text("SHUFFLE PLAY"))
			} else {
				shufflePlayButton = d.Object(androidui.ID(childrenID), androidui.ClassName("android.widget.LinearLayout"))
			}

			if err := waitAndClickObject(ctx, shufflePlayButton, "shuffle play button", defaultUITimeout); err != nil {
				testing.ContextLog(ctx, "Shuffle play button doesn't exists")
			}

			if err := pauseButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return errors.Wrap(err, "the pause button doesn't exists")
			}
			return nil
		}

		if err = recorder.Run(ctx, func(ctx context.Context) error {
			launchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if _, err := ash.GetARCAppWindowInfo(ctx, tconn, spotifyPackageName); err == nil {
				testing.ContextLogf(ctx, "Package %s is already visible, skipping", spotifyPackageName)
				return nil
			}

			var startTime time.Time
			// Sometimes the Spotify App will fail to open, so add retry here.
			if err := testing.Poll(launchCtx, func(ctx context.Context) error {
				if err := launcher.SearchAndLaunch(tconn, kb, Spotify)(ctx); err != nil {
					return errors.Wrapf(err, "failed to launch %s app", Spotify)
				}
				startTime = time.Now()
				return ash.WaitForVisible(ctx, tconn, spotifyPackageName)
			}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
				return errors.Wrapf(err, "failed to wait for the new window of %s", spotifyPackageName)
			}
			if Spotify == appName {
				endTime := time.Now()
				appStartTime = endTime.Sub(startTime).Milliseconds()
			}
			testing.ContextLog(ctx, "Start to play Spotify")
			if err = playSotify(launchCtx); err != nil {
				return errors.Wrap(err, "failed to play Spotify")
			}
			// Waits some time to stabilize the result of launcher animations.
			return testing.Sleep(launchCtx, timeout)
		}); err != nil {
			return errors.Wrap(err, "failed to launch Spotify")
		}
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
			// We don't need to keep the connection, so close it before leaving this function.
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
		return nil
	}

	switchTabs := func(ctx context.Context, pages []string) error {
		if err := setVolume(ctx, tconn, initialVolume); err != nil {
			return errors.Wrap(err, "failed to set volume")
		}

		for tabIdx := 0; tabIdx < len(pages); tabIdx++ {
			testing.ContextLog(ctx, "Switching Chrome tab")
			if err := uiHandler.SwitchToChromeTabByIndex(tabIdx)(ctx); err != nil {
				return errors.Wrap(err, "failed to switch tab")
			}
			testing.ContextLog(ctx, "Volume up")
			kb.Accel(ctx, topRow.VolumeUp)
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
		for i := 0; i < len(ws); i++ {
			// Cycle through all windows, find out the browser window, and do tab switch.

			// Switch to the least recent window to start from the window we opened first.
			testing.ContextLog(ctx, "Switching window by overview")
			if err := uiHandler.SwitchToLRUWindow(cuj.SwitchWindowThroughOverview)(ctx); err != nil {
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
			if err := switchTabs(ctx, pageList[browserWinIdx]); err != nil {
				return errors.Wrap(err, "failed to switch tabs")
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
		return switchAllBrowserTabs(ctx)
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
			if err := setVolume(ctx, tconn, initialVolume); err != nil {
				return errors.Wrap(err, "failed to set os volume")
			}
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
	testing.ContextLog(ctx, "Take photo and video")
	if err := recorder.Run(ctx, func(ctx context.Context) error { return takePhotoAndVideo(ctx, cr, ccaScriptPaths, outDir) }); err != nil {
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
	if err = pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to store values")
	}
	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}
	return nil
}

func setVolume(ctx context.Context, tconn *chrome.TestConn, volume int) (err error) {
	testing.ContextLog(ctx, "Set volume to ", volume)

	javascrpt := fmt.Sprintf(`new Promise((resolve, reject) => {
		const adjustVolume = level => {
			chrome.audio.getDevices({ streamTypes: ['OUTPUT'], isActive: true }, devices => { chrome.audio.setProperties(devices[0].id, { level }, () => { }) });
		};
		adjustVolume(%d);
		resolve();
	});`, volume)
	if err = tconn.EvalPromise(ctx, javascrpt, nil); err != nil {
		return errors.Wrap(err, "failed to set operation system sound volume level")
	}
	return
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

	// Take Photo
	_, err = app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		return errors.Wrap(err, "failed to take single photo")
	}
	// Record video
	testing.ContextLog(ctx, "Switch to video mode")
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to video mode")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after switch to video mode")
	}
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "failed to click shutter")
	}
	if err := app.WaitForState(ctx, "recording", true); err != nil {
		return errors.Wrap(err, "recording is not started")
	}
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	testing.ContextLog(ctx, "Stopping a video")
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "failed to click shutter")
	}
	return nil
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
