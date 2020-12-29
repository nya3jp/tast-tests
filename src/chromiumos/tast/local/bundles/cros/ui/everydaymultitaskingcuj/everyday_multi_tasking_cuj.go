// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package everydaymultitaskingcuj contains the test code for EverydayMultiTaskingCUJ.
// The test is extracted into this package to be shared between EverydayMultiTaskingCUJ,
// EverydayMultiTaskingCUJPlaymusic and EverydayMultiTaskingCUJSpotify.
package everydaymultitaskingcuj

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/cuj/bluetooth"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
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
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, tier cuj.Tier, appName string, tabletMode, enableBT bool) {
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

	isBtEnabled, err := bluetooth.IsEnabled(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth status: ", err)
	}

	if enableBT {
		s.Log("Start to connect bluetooth")
		deviceName := s.RequiredVar("ui.bt_devicename")
		if err := bluetooth.ConnectDevice(ctx, deviceName); err != nil {
			s.Fatal("Failed to connect bluetooth: ", err)
		}
		if !isBtEnabled {
			defer func() {
				if err := bluetooth.Disable(ctx); err != nil {
					s.Fatal("Failed to disable bluetooth: ", err)
				}
			}()
		}
	} else if isBtEnabled {
		s.Log("Start to disable bluetooth")
		if err := bluetooth.Disable(ctx); err != nil {
			s.Fatal("Failed to disable bluetooth: ", err)
		}
		defer func() {
			if err := bluetooth.Enable(ctx); err != nil {
				s.Fatal("Failed to connect bluetooth: ", err)
			}
		}()
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure the tablet mode state: ", err)
	}
	defer cleanup(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	var pc pointer.Controller
	var setOverviewModeAndWait func(ctx context.Context) error
	type subtest struct {
		name string
		desc string
		f    func(ctx context.Context, s *testing.State, i int) error
	}
	browserWindows := map[int]bool{}
	var ws []*ash.Window
	var subtest2 subtest
	if tabletMode {
		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
		pc = tc
		stw := tc.EventWriter()
		tsew := tc.Touchscreen()
		setOverviewModeAndWait = func(ctx context.Context) error {
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				s.Log("Set Overview Mode And Wait")
				return ash.DragToShowOverview(ctx, tsew, stw, tconn)
			}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
				return err
			}
			return nil
		}
		subtest2 = subtest{
			"hotseat",
			"Switching the focused window through clicking the hotseat",
			func(ctx context.Context, s *testing.State, i int) error {
				// In this subtest, update the active window through hotseat. First,
				// swipe-up quickly to reveal the hotseat, and then tap the app icon
				// for the next active window. In case there are multiple windows in
				// an app, it will show up a pop-up, so tap on the menu item.
				tcc := tc.TouchCoordConverter()
				if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
					return errors.Wrap(err, "failed to show the hotseat")
				}
				if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to wait for location changes")
				}

				// Get the bounds of the shelf icons. The shelf icon bounds are
				// available from ScrollableShelfInfo, while the metadata for ShelfItems
				// are in another place (ShelfItem).  Use ShelfItem to filter out
				// the apps with no windows, and fetch its icon bounds from
				// ScrollableShelfInfo.
				items, err := ash.ShelfItems(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "failed to obtain the shelf items")
				}
				shelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
				if err != nil {
					return errors.Wrap(err, "failed to obtain the shelf UI info")
				}
				if len(items) != len(shelfInfo.IconsBoundsInScreen) {
					return errors.Errorf("mismatch count: %d vs %d", len(items), len(shelfInfo.IconsBoundsInScreen))
				}

				iconBounds := make([]coords.Rect, 0, len(items))
				hasYoutubeIcon := false
				for i, item := range items {
					if item.Status == ash.ShelfItemClosed {
						continue
					}
					if strings.HasPrefix(strings.ToLower(item.Title), "youtube") {
						hasYoutubeIcon = true
					}
					iconBounds = append(iconBounds, *shelfInfo.IconsBoundsInScreen[i])
				}

				// browserPopupItemsCount is the number of browser windows to be chosen
				// from the popup menu shown by tapping the browser icon. Basically all
				// of the browser windows should be there, but when youtube icon
				// appears, youtube should be chosen from its own icon, so the number
				// should be decremented.
				browserPopupItemsCount := len(browserWindows)
				if hasYoutubeIcon {
					browserPopupItemsCount--
				}

				// Find the correct-icon for i-th run. Assumptions:
				// - each app icon has 1 window, except for the browser icon (there are len(browserWindows))
				// - browser icon is the leftmost (iconIdx == 0)
				// With these assumptions, it selects the icons from the right, and
				// when it reaches to browser icons, it selects a window from the popup
				// menu from the top. In other words, there would be icons of
				// [browser] [play store] [gmail] ...
				// and selecting [gmail] -> [play store] -> [browser]
				// and selecting browser icon shows a popup.
				iconIdx := len(ws) - (browserPopupItemsCount - 1) - i - 1
				var isPopup bool
				var popupIdx int
				if iconIdx <= 0 {
					isPopup = true
					// This assumes the order of menu items of window seleciton popup is
					// stable. Selecting from the top, but offset-by-one since the first
					// menu item is just a title, not clickable.
					popupIdx = -iconIdx
					iconIdx = 0
				}
				if err := pointer.Click(ctx, tc, iconBounds[iconIdx].CenterPoint()); err != nil {
					return errors.Wrapf(err, "failed to click icon at %d", iconIdx)
				}
				if isPopup {
					menuFindParams := ui.FindParams{ClassName: "MenuItemView"}
					if err := ui.WaitUntilExists(ctx, tconn, menuFindParams, 10*time.Second); err != nil {
						return errors.Wrap(err, "expected to see menu items, but not seen")
					}
					menus, err := ui.FindAll(ctx, tconn, menuFindParams)
					if err != nil {
						return errors.Wrap(err, "can't find the menu items")
					}
					defer menus.Release(ctx)
					targetMenus := make([]*ui.Node, 0, len(menus))
					for i := 1; i < len(menus); i++ {
						if !hasYoutubeIcon || !strings.HasPrefix(strings.ToLower(menus[i].Name), "youtube") {
							targetMenus = append(targetMenus, menus[i])
						}
					}
					if err := pointer.Click(ctx, tc, targetMenus[popupIdx].Location.CenterPoint()); err != nil {
						return errors.Wrapf(err, "failed to click menu item %d", popupIdx)
					}
				}
				if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to wait for location changes")
				}
				return ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden)
			},
		}
	} else {
		pc = pointer.NewMouseController(tconn)
		setOverviewModeAndWait = func(ctx context.Context) error {
			if err := kb.Accel(ctx, topRow.SelectTask); err != nil {
				return errors.Wrap(err, "failed to hit overview key")
			}
			return ash.WaitForOverviewState(ctx, tconn, ash.Shown, timeout)
		}
		subtest2 = subtest{
			"alt-tab",
			"Switching the focused window through Alt-Tab",
			func(ctx context.Context, s *testing.State, i int) error {
				// Press alt -> hit tabs for the number of windows to choose the last used
				// window -> release alt.
				if err := kb.AccelPress(ctx, "Alt"); err != nil {
					return errors.Wrap(err, "failed to press alt")
				}
				defer kb.AccelRelease(ctx, "Alt")
				if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
				for j := 0; j < len(ws)-1; j++ {
					if err := kb.Accel(ctx, "Tab"); err != nil {
						return errors.Wrap(err, "failed to type tab")
					}
					if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
						return errors.Wrap(err, "failed to wait")
					}
				}
				if err := testing.Sleep(ctx, time.Second); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
				return nil
			},
		}
	}
	defer pc.Close()

	// Install android apps for the everyday works: Spotify.
	if appName == Spotify {
		func() {
			s.Log("Check and install ", spotifyPackageName)
			installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()
			if err = playstore.InstallApp(installCtx, a, d, spotifyPackageName, -1); err != nil {
				s.Fatalf("Failed to install %s: %v", spotifyPackageName, err)
			}
			if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
				s.Fatal("Failed to close Play Store: ", err)
			}
		}()
	}

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

	// Launch arc apps from the app launcher; first open the app-launcher, type
	// the query and select the first search result, and wait for the app window
	// to appear. When the app has the splash screen, skip it.
	for _, app := range []struct {
		query       string
		packageName string
		skipSplash  func(ctx context.Context) error
	}{
		{Spotify, spotifyPackageName, func(ctx context.Context) error {
			const (
				spotifyIDPrefix   = "com.spotify.music:id/"
				playPauseButtonID = spotifyIDPrefix + "play_pause_button"
				searchTabID       = spotifyIDPrefix + "search_tab"
				searchFieldID     = spotifyIDPrefix + "find_search_field_text"
				queryID           = spotifyIDPrefix + "query"
				childrenID        = spotifyIDPrefix + "children"
				albumName         = "Photograph"
				singerName        = "Song • Ed Sheeran"
				DefaultUITimeout  = 30 * time.Second
				waitTime          = 3 * time.Second
			)
			fisrtLogin := false
			signIn := d.Object(androidui.Text("Continue with Google"))
			if err := signIn.WaitForExists(ctx, waitTime); err != nil {
				s.Log(`Failed to find "Continue with Google" button`)
			} else if err := signIn.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "Continue with Google" button`)
			} else {
				account := s.RequiredVar("ui.cuj_username")
				accountButton := d.Object(androidui.Text(account))
				if err := waitAndClickObject(ctx, accountButton, "account button", waitTime); err != nil {
					s.Log("Sign in directly")
				}
				fisrtLogin = true
			}

			dismiss := d.Object(androidui.Text("DISMISS"))
			if err := dismiss.WaitForExists(ctx, waitTime); err != nil {
				s.Log(`Failed to find "DISMISS" button, believing splash screen has been dismissed already`)
			} else if err := dismiss.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "DISMISS" button`)
			}

			promp := d.Object(androidui.Text("NO, THANKS"))
			if err := promp.WaitForExists(ctx, waitTime); err != nil {
				s.Log(`Failed to find "NO, THANKS" button`)
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
					return errors.Wrap(err, `failed to click "play button" `)
				}
				if err := pauseButton.WaitForExists(ctx, timeout); err != nil {
					testing.ContextLog(ctx, "The pause button doesn't exists")
				} else {
					return nil
				}
			}

			searchTab := d.Object(androidui.ID(searchTabID))
			if err := waitAndClickObject(ctx, searchTab, "search tab", DefaultUITimeout); err != nil {
				return err
			}

			searchField := d.Object(androidui.ID(searchFieldID))
			if err := waitAndClickObject(ctx, searchField, "search feild", DefaultUITimeout); err != nil {
				return err
			}

			query := d.Object(androidui.ID(queryID))
			if err := waitAndClickObject(ctx, query, "query feild", DefaultUITimeout); err != nil {
				return err
			}

			if err := kb.Type(ctx, albumName); err != nil {
				return errors.Wrap(err, "failed to type album")
			}

			singerButton := d.Object(androidui.Text(singerName))
			if err := waitAndClickObject(ctx, singerButton, "singerButton", DefaultUITimeout); err != nil {
				return err
			}

			var shufflePlayButton *androidui.Object
			if fisrtLogin {
				shufflePlayButton = d.Object(androidui.Text("SHUFFLE PLAY"))
			} else {
				shufflePlayButton = d.Object(androidui.ID(childrenID), androidui.ClassName("android.widget.LinearLayout"))
			}

			if err := waitAndClickObject(ctx, shufflePlayButton, "shuffle play button", DefaultUITimeout); err != nil {
				testing.ContextLog(ctx, "Shuffle play button doesn't exists")
			}

			if err := pauseButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
				return errors.Wrap(err, "the pause button doesn't exists")
			}
			return nil
		}},
	} {
		if appName == Spotify {
			if err = recorder.Run(ctx, func(ctx context.Context) error {
				launchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				if _, err := ash.GetARCAppWindowInfo(ctx, tconn, app.packageName); err == nil {
					testing.ContextLogf(ctx, "Package %s is already visible, skipping", app.packageName)
					return nil
				}
				if err := launcher.SearchAndLaunch(tconn, kb, app.query)(ctx); err != nil {
					return errors.Wrapf(err, "failed to launch %s app", app.query)
				}
				startTime := time.Now()
				if err := testing.Poll(launchCtx, func(ctx context.Context) error {
					return ash.WaitForVisible(ctx, tconn, app.packageName)
				}, &testing.PollOptions{Timeout: 2 * time.Minute}); err != nil {
					return errors.Wrapf(err, "failed to wait for the new window of %s", app.packageName)
				}
				if app.query == appName {
					endTime := time.Now()
					appStartTime = endTime.Sub(startTime).Milliseconds()
				}
				s.Log("Skipping the splash screen of ", app.query)
				if err = app.skipSplash(launchCtx); err != nil {
					return errors.Wrap(err, "failed to skip the splash screen of the app")
				}
				// Waits some time to stabilize the result of launcher animations.
				return testing.Sleep(launchCtx, timeout)
			}); err != nil {
				s.Fatalf("Failed to launch %s: %v", app.query, err)
			}
		}
	}
	openNewTab := func(subURL string) error {
		// Open new tab
		kb.Accel(ctx, "ctrl+t")
		subConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
		if err != nil {
			return errors.Wrap(err, "failed to find new tab")
		}
		defer subConn.Close()
		if err := subConn.Navigate(ctx, subURL); err != nil {
			return errors.Wrapf(err, "failed to navigate to %s", subURL)
		}
		if err := webutil.WaitForQuiescence(ctx, subConn, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for page to finish loading")
		}
		return nil
	}

	openBrowserWithTabs := func(urlList []string) error {
		url := urlList[0]
		conn, err := cr.NewConn(ctx, url, cdputil.WithNewWindow())
		if err != nil {
			return errors.Wrapf(err, "failed to open %s", url)
		}
		defer conn.Close()

		for _, subURL := range urlList[1:] {
			if err := openNewTab(subURL); err != nil {
				return errors.Wrapf(err, "failed to open page %s", subURL)
			}

			if appName == YoutubeMusic && subURL == youtubeMusicURL {
				shuffleParams := ui.FindParams{
					Name: "Shuffle",
					Role: ui.RoleTypeButton,
				}
				pauseParams := ui.FindParams{
					Name: "Pause",
					Role: ui.RoleTypeButton,
				}
				if err := testing.Poll(ctx, func(ctx context.Context) error {
					if err := cuj.WaitAndClick(ctx, tconn, shuffleParams, time.Second*5); err != nil {
						return errors.Wrap(err, "failed to click shffle button: ")
					}
					_, err = ui.FindWithTimeout(ctx, tconn, pauseParams, time.Second*15)
					if err != nil {
						return errors.Wrap(err, "failed to find pause button: ")
					}
					return nil
				}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
					s.Fatal("Failed to play youtube music: ", err)
				}
			}
		}

		// We don't need to keep the connection, so close it now.
		if err = conn.Close(); err != nil {
			return errors.Wrapf(err, "failed to close the connection to %s", url)
		}
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			if w.WindowType != ash.WindowTypeBrowser {
				return false
			}
			return !browserWindows[w.ID]
		})
		if err != nil {
			return errors.Wrapf(err, "failed to find the browser window for %s", url)
		}
		browserWindows[w.ID] = true
		if !tabletMode {
			if _, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventNormal); err != nil {
				return errors.Wrapf(err, "ailed to change the window (%s) into the normal state", url)
			}
		}
		return nil
	}

	switchTabs := func(ctx context.Context, s *testing.State, pages []string) error {
		if err := setVolume(ctx, tconn, initialVolume); err != nil {
			return errors.Wrap(err, "failed to set volume")
		}

		for i := 0; i < len(pages); i++ {
			s.Log("Volume up")
			kb.Accel(ctx, topRow.VolumeUp)
			kb.Accel(ctx, "ctrl+tab")
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
		}
		return nil
	}

	switchAllBrowserTabs := func(ctx context.Context, s *testing.State) error {
		s.Log("Start to switch all browser tabs")
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain the window list")
		}
		browserCount := 0
		for _, w := range ws {
			if w.WindowType == "Browser" {
				browserCount++
			}
		}

		switchBrowserTabs := func(ctx context.Context, i int) error {
			// Switch browser through the overview mode
			if err := setOverviewModeAndWait(ctx); err != nil {
				return errors.Wrap(err, "failed to enter into the overview mode")
			}
			done := false
			defer func() {
				// In case of errornerous operations; finish the overview mode.
				if !done {
					if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
						s.Error("Failed to finish the overview mode: ", err)
					}
				}
			}()
			ws, err = ash.GetAllWindows(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to obtain the window list")
			}
			var targetWindow *ash.Window
			b := 0
			for _, w := range ws {
				if w.WindowType == "Browser" {
					b++
					if b == browserCount {
						targetWindow = w
					}
				}
			}
			if err := pointer.Click(ctx, pc, targetWindow.OverviewInfo.Bounds.CenterPoint()); err != nil {
				return errors.Wrap(err, "failed to click")
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == targetWindow.ID && w.OverviewInfo == nil && w.IsActive
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
				s.Error("Failed to finish the overview mode: ", err)
			}
			done = true

			if err := switchTabs(ctx, s, pageList[i]); err != nil {
				return errors.Wrap(err, "failed to switch tabs")
			}
			return nil
		}
		for i := 0; i < browserCount; i++ {
			if err := switchBrowserTabs(ctx, i); err != nil {
				return errors.Wrap(err, "failed to switch browser tabs")
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
		return switchAllBrowserTabs(ctx, s)
	}); err != nil {
		s.Fatal("Failed to run the open tabs and switch tabs scenario: ", err)
	}

	subtests := []subtest{
		{
			"overview",
			"Switching the focused window through the overview mode",
			func(ctx context.Context, s *testing.State, i int) error {
				if err := setOverviewModeAndWait(ctx); err != nil {
					return errors.Wrap(err, "failed to enter into the overview mode")
				}
				done := false
				defer func() {
					// In case of errornerous operations; finish the overview mode.
					if !done {
						if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
							s.Error("Failed to finish the overview mode: ", err)
						}
					}
				}()
				ws, err := ash.GetAllWindows(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "failed to get the overview windows")
				}
				// Find the bottom-right overview item; which is the bottom of the LRU
				// list of the windows.
				var targetWindow *ash.Window
				for _, w := range ws {
					if w.OverviewInfo == nil {
						continue
					}
					if targetWindow == nil {
						targetWindow = w
					} else {
						overviewBounds := w.OverviewInfo.Bounds
						targetBounds := targetWindow.OverviewInfo.Bounds
						// Assumes the window is arranged in the grid and pick up the bottom
						// right one.
						if overviewBounds.Top > targetBounds.Top || (overviewBounds.Top == targetBounds.Top && overviewBounds.Left > targetBounds.Left) {
							targetWindow = w
						}
					}
				}
				if targetWindow == nil {
					return errors.New("no windows are in overview mode")
				}
				if err := pointer.Click(ctx, pc, targetWindow.OverviewInfo.Bounds.CenterPoint()); err != nil {
					return errors.Wrap(err, "failed to click")
				}
				if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == targetWindow.ID && w.OverviewInfo == nil && w.IsActive
				}, &testing.PollOptions{Timeout: timeout}); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
				done = true
				return nil
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
				if err := setVolume(ctx, tconn, initialVolume); err != nil {
					return errors.Wrap(err, "failed to set os volume")
				}
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
	s.Log("Take photo and video")
	if err := recorder.Run(ctx, func(ctx context.Context) error { return takePhotoAndVideo(ctx, s, cr) }); err != nil {
		s.Fatal("Failed to run the camera scenario: ", err)
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

func contains(list []string, searchterm string) bool {
	for _, s := range list {
		if s == searchterm {
			return true
		}
	}
	return false
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

func takePhotoAndVideo(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
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
