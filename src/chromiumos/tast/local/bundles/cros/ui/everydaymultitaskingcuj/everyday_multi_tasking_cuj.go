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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/cuj/bluetooth"
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

	if appName == Spotify {
		skipSplash := func(ctx context.Context) error {
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
		}

		if err = recorder.Run(ctx, func(ctx context.Context) error {
			launchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if _, err := ash.GetARCAppWindowInfo(ctx, tconn, spotifyPackageName); err == nil {
				testing.ContextLogf(ctx, "Package %s is already visible, skipping", spotifyPackageName)
				return nil
			}
			if err := launcher.SearchAndLaunch(tconn, kb, Spotify)(ctx); err != nil {
				return errors.Wrapf(err, "failed to launch %s app", Spotify)
			}
			startTime := time.Now()
			if err := testing.Poll(launchCtx, func(ctx context.Context) error {
				return ash.WaitForVisible(ctx, tconn, spotifyPackageName)
			}, &testing.PollOptions{Timeout: 2 * time.Minute}); err != nil {
				return errors.Wrapf(err, "failed to wait for the new window of %s", spotifyPackageName)
			}
			if Spotify == appName {
				endTime := time.Now()
				appStartTime = endTime.Sub(startTime).Milliseconds()
			}
			s.Log("Skipping the splash screen of ", Spotify)
			if err = skipSplash(launchCtx); err != nil {
				return errors.Wrap(err, "failed to skip the splash screen of the app")
			}
			// Waits some time to stabilize the result of launcher animations.
			return testing.Sleep(launchCtx, timeout)
		}); err != nil {
			s.Fatalf("Failed to launch %s: %v", Spotify, err)
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
