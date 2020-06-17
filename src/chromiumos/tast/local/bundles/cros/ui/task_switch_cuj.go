// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      8 * time.Minute,
		Vars: []string{
			"mute",
			"ui.TaskSwitchCUJ.username",
			"ui.TaskSwitchCUJ.password",
			"ui.cuj_username",
			"ui.cuj_password",
		},
		Pre: cuj.LoggedInToCUJUser(),
		Params: []testing.Param{
			{Val: false},
			{
				Name:              "tablet_mode",
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Val:               true,
			},
		},
	})
}

func TaskSwitchCUJ(ctx context.Context, s *testing.State) {
	const (
		searchIconID         = "com.android.vending:id/search_bar"
		playStorePackageName = "com.android.vending"
		gmailPackageName     = "com.google.android.gm"
		timeout              = 10 * time.Second
	)

	cr := s.PreValue().(cuj.PreData).Chrome
	a := s.PreValue().(cuj.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kw.Close()

	tabletMode := s.Param().(bool)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure the tablet mode state: ", err)
	}
	defer cleanup(ctx)

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close()

	// Install android apps for the everyday works: Gmail.
	// Google Calendar and youtube are not installed to reduce the flakiness
	// around the app installation.
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("Failed to list the installed packages: ", err)
	}
	for _, pkgName := range []string{gmailPackageName} {
		if _, ok := pkgs[pkgName]; ok {
			s.Logf("%s is already installed", pkgName)
			continue
		}
		s.Log("Installing ", pkgName)
		installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		if err = playstore.InstallApp(installCtx, a, d, pkgName); err != nil {
			cancel()
			s.Fatalf("Failed to install %s: %v", pkgName, err)
		}
		cancel()
	}

	if err = cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for idle-ness: ", err)
	}

	var pc pointer.Controller
	var setOverviewModeAndWait func(ctx context.Context) error
	var openAppList func(ctx context.Context) error
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
			s.Fatal("Failed to create a touch controller")
		}
		pc = tc
		stw := tc.EventWriter()
		tsew := tc.Touchscreen()
		setOverviewModeAndWait = func(ctx context.Context) error {
			return ash.DragToShowOverview(ctx, tsew.Width(), tsew.Height(), stw, tconn)
		}
		openAppList = func(ctx context.Context) error {
			return ash.DragToShowHomescreen(ctx, tsew.Width(), tsew.Height(), stw, tconn)
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
				if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
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
					menuFindParams := chromeui.FindParams{ClassName: "MenuItemView"}
					if err := chromeui.WaitUntilExists(ctx, tconn, menuFindParams, 10*time.Second); err != nil {
						return errors.Wrap(err, "expected to see menu items, but not seen")
					}
					menus, err := chromeui.FindAll(ctx, tconn, menuFindParams)
					if err != nil {
						return errors.Wrap(err, "can't find the menu items")
					}
					defer menus.Release(ctx)
					targetMenus := make([]*chromeui.Node, 0, len(menus))
					for i := 1; i < len(menus); i++ {
						if !hasYoutubeIcon || !strings.HasPrefix(strings.ToLower(menus[i].Name), "youtube") {
							targetMenus = append(targetMenus, menus[i])
						}
					}
					if err := pointer.Click(ctx, tc, targetMenus[popupIdx].Location.CenterPoint()); err != nil {
						return errors.Wrapf(err, "failed to click menu item %d", popupIdx)
					}
				}
				if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to wait for location changes")
				}
				return ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden)
			},
		}
	} else {
		pc = pointer.NewMouseController(tconn)
		topRow, err := input.KeyboardTopRowLayout(ctx, kw)
		if err != nil {
			s.Fatal("Failed to obtain the top-row layout: ", err)
		}
		setOverviewModeAndWait = func(ctx context.Context) error {
			if err := kw.Accel(ctx, topRow.SelectTask); err != nil {
				return errors.Wrap(err, "failed to hit overview key")
			}
			return ash.WaitForOverviewState(ctx, tconn, ash.Shown, timeout)
		}
		openAppList = func(ctx context.Context) error {
			return kw.Accel(ctx, "search")
		}
		subtest2 = subtest{
			"alt-tab",
			"Switching the focused window through Alt-Tab",
			func(ctx context.Context, s *testing.State, i int) error {
				// Press alt -> hit tabs for the number of windows to choose the last used
				// window -> release alt.
				if err := kw.AccelPress(ctx, "Alt"); err != nil {
					return errors.Wrap(err, "failed to press alt")
				}
				defer kw.AccelRelease(ctx, "Alt")
				if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
				for j := 0; j < len(ws)-1; j++ {
					if err := kw.Accel(ctx, "Tab"); err != nil {
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

	// Set up the cuj.Recorder: this test will measure the combinations of
	// animation smoothness for window-cycles (alt-tab selection), launcher,
	// and overview.
	configs := []cuj.MetricConfig{
		cuj.NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Container"),
		cuj.NewLatencyMetricConfig("Ash.DragWindowFromShelf.PresentationTime"),
		cuj.NewSmoothnessMetricConfig("Ash.Homescreen.AnimationSmoothness"),
		cuj.NewLatencyMetricConfig("Ash.HotseatTransition.Drag.PresentationTime"),
	}
	for _, suffix := range []string{"HideLauncherForWindow", "EnterFullscreenAllApps", "EnterFullscreenSearch", "FadeInOverview", "FadeOutOverview"} {
		configs = append(configs, cuj.NewSmoothnessMetricConfig(
			"Apps.HomeLauncherTransition.AnimationSmoothness."+suffix))
	}
	for _, state := range []string{"Peeking", "Close", "Half"} {
		configs = append(configs, cuj.NewSmoothnessMetricConfig(
			"Apps.StateTransition.AnimationSmoothness."+state+".ClamshellMode"))
	}
	for _, suffix := range []string{"SingleClamshellMode", "ClamshellMode", "TabletMode"} {
		configs = append(configs,
			cuj.NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter."+suffix),
			cuj.NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit."+suffix),
		)
	}
	for _, suffix := range []string{"TransitionToShownHotseat", "TransitionToExtendedHotseat", "TransitionToHiddenHotseat"} {
		configs = append(configs,
			cuj.NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness."+suffix))
	}
	recorder, err := cuj.NewRecorder(ctx, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	// Launch arc apps from the app launcher; first open the app-launcher, type
	// the query and select the first search result, and wait for the app window
	// to appear. When the app has the splash screen, skip it.
	// TODO(mukai): make sure that the initial search result is the intended
	// one.
	for _, app := range []struct {
		query       string
		packageName string
		skipSplash  func(ctx context.Context) error
	}{
		{"play store", playStorePackageName, func(ctx context.Context) error {
			return nil
		}},
		{"gmail", gmailPackageName, func(ctx context.Context) error {
			const (
				dialogID            = "com.google.android.gm:id/customPanel"
				dismissID           = "com.google.android.gm:id/gm_dismiss_button"
				customPanelMaxCount = 10
			)
			gotIt := d.Object(ui.TextMatches("GOT IT"))
			if err := gotIt.WaitForExists(ctx, timeout); err != nil {
				s.Log(`Failed to find "GOT IT" button, believing splash screen has been dismissed already`)
				return nil
			}
			if err := gotIt.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "GOT IT" button`)
			}
			// Sometimes, the account information might not be ready yet. In that case
			// a warning dialog appears. If the warning message does not appear, it
			// is fine.
			pleaseAdd := d.Object(ui.TextMatches("Please add at least one email address"))
			if err := pleaseAdd.WaitForExists(ctx, timeout); err == nil {
				// Even though the warning dialog appears, the email address should
				// appear already. Therefore, here simply clicks the 'OK' button to
				// dismiss the warning dialog and moves on.
				if err := testing.Sleep(ctx, timeout); err != nil {
					return errors.Wrap(err, "failed to wait for the email address appearing")
				}
				okButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text("OK"))
				if err := okButton.Exists(ctx); err != nil {
					return errors.Wrap(err, "failed to find the ok button")
				}
				if err := okButton.Click(ctx); err != nil {
					return errors.Wrap(err, "failed to click the OK button")
				}
			}
			takeMe := d.Object(ui.TextMatches("TAKE ME TO GMAIL"))
			if err := takeMe.WaitForExists(ctx, timeout); err != nil {
				return errors.Wrap(err, `"TAKE ME TO GMAIL" is not shown`)
			}
			if err := takeMe.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "TAKE ME TO GMAIL" button`)
			}
			// After clicking 'take me to gmail', it might show a series of dialogs to
			// finalize the setup. Here skips those dialogs by clicking their 'ok'
			// buttons.
			for i := 0; i < customPanelMaxCount; i++ {
				dialog := d.Object(ui.ID(dialogID))
				if err := dialog.WaitForExists(ctx, timeout); err != nil {
					return nil
				}
				dismiss := d.Object(ui.ID(dismissID))
				if err := dismiss.Exists(ctx); err != nil {
					return errors.Wrap(err, "dismiss button not found")
				}
				if err := dismiss.Click(ctx); err != nil {
					return errors.Wrap(err, "failed to click the dismiss button")
				}
			}
			return errors.New("too many dialog popups")
		}},
	} {
		if err = recorder.Run(ctx, tconn, func() error {
			launchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if _, err := ash.GetARCAppWindowInfo(ctx, tconn, app.packageName); err == nil {
				testing.ContextLogf(ctx, "Package %s is already visible, skipping", app.packageName)
				return nil
			}
			if err := openAppList(launchCtx); err != nil {
				return errors.Wrap(err, "failed to open the app-list")
			}
			if err := testing.Sleep(launchCtx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.Type(launchCtx, app.query); err != nil {
				return errors.Wrap(err, "failed to type the query")
			}
			if err := testing.Sleep(launchCtx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.Accel(launchCtx, "enter"); err != nil {
				return errors.Wrap(err, "failed to type the enter key")
			}
			if err := ash.WaitForVisible(launchCtx, tconn, app.packageName); err != nil {
				return errors.Wrapf(err, "failed to wait for the new window of %s", app.packageName)
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

	// Here adds browser windows:
	// 1. webGL aquarium -- adding considerable load on graphics.
	// 2. chromium issue tracker -- considerable amount of elements.
	// 3. youtube -- the substitute of youtube app.
	for _, url := range []string{
		"https://webglsamples.org/aquarium/aquarium.html",
		"https://bugs.chromium.org/p/chromium/issues/list",
		"https://youtube.com/",
	} {
		conn, err := cr.NewConn(ctx, url, cdputil.WithNewWindow())
		if err != nil {
			s.Fatalf("Failed to open %s: %v", url, err)
		}
		// We don't need to keep the connection, so close it now.
		if err = conn.Close(); err != nil {
			s.Fatalf("Failed to close the connection to %s: %v", url, err)
		}
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			if w.WindowType != ash.WindowTypeBrowser {
				return false
			}
			return !browserWindows[w.ID]
		})
		if err != nil {
			s.Fatalf("Failed to find the browser window for %s: %v", url, err)
		}
		browserWindows[w.ID] = true
		if !tabletMode {
			if _, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventNormal); err != nil {
				s.Fatalf("Failed to change the window (%s) into the normal state: %v", url, err)
			}
		}
	}

	if err = cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for idle-ness: ", err)
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
			for i := 0; i < len(ws); i++ {
				if err := recorder.Run(ctx, tconn, func() error { return st.f(ctx, s, i) }); err != nil {
					s.Error("Failed to run the scenario: ", err)
				}
			}
		})
	}

	pv := perf.NewValues()
	if err = recorder.Record(pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
