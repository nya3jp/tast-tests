// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type taskSWitchCUJTestParam struct {
	tablet    bool
	useLacros bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      8 * time.Minute,
		Vars:         []string{"mute"},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "loggedInToCUJUser",
				Val:               taskSWitchCUJTestParam{},
			},
			{
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "loggedInToCUJUser",
				Val:               taskSWitchCUJTestParam{},
			},
			{
				Name:              "tablet_mode",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "loggedInToCUJUser",
				Val: taskSWitchCUJTestParam{
					tablet: true,
				},
			},
			{
				Name:              "tablet_mode_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val: taskSWitchCUJTestParam{
					tablet: true,
				},
			},
			{
				Name:              "lacros_clamshell",
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
				ExtraData:         []string{launcher.DataArtifact},
				Fixture:           "loggedInToCUJUserLacrosWithARC",
				Val: taskSWitchCUJTestParam{
					useLacros: true,
				},
			},
			{
				Name:              "lacros_clamshell_vm",
				ExtraSoftwareDeps: []string{"android_vm", "lacros"},
				ExtraData:         []string{launcher.DataArtifact},
				Fixture:           "loggedInToCUJUserLacrosWithARC",
				Val: taskSWitchCUJTestParam{
					useLacros: true,
				},
			},
			{
				Name:              "lacros_tablet",
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
				ExtraData:         []string{launcher.DataArtifact},
				Fixture:           "loggedInToCUJUserLacrosWithARC",
				Val: taskSWitchCUJTestParam{
					useLacros: true,
					tablet:    true,
				},
			},
			{
				Name:              "lacros_tablet_vm",
				ExtraSoftwareDeps: []string{"android_vm", "lacros"},
				ExtraData:         []string{launcher.DataArtifact},
				Fixture:           "loggedInToCUJUserLacrosWithARC",
				Val: taskSWitchCUJTestParam{
					useLacros: true,
					tablet:    true,
				},
			},
		},
	})
}

func TaskSwitchCUJ(ctx context.Context, s *testing.State) {
	const (
		playStorePackageName = "com.android.vending"
		gmailPackageName     = "com.google.android.gm"
		timeout              = 10 * time.Second
	)

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	testParam := s.Param().(taskSWitchCUJTestParam)

	a := s.FixtValue().(cuj.FixtureData).ARC

	var tconn *chrome.TestConn
	var cs ash.ConnSource

	{
		// Keep `cr` inside to avoid accidental access of ash-chrome in lacros
		// variation.
		cr := s.FixtValue().(cuj.FixtureData).Chrome
		if !testParam.useLacros {
			cs = cr
		}

		var err error
		tconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect to the test API connection: ", err)
		}
	}

	if testParam.useLacros {
		// Launch lacros via shelf.
		f := s.FixtValue().(cuj.FixtureData).LacrosFixt

		// TODO(crbug.com/1127165): Remove this when we can use Data in fixtures.
		l, err := lacros.ShelfLaunch(ctx, tconn, f, s.DataPath(launcher.DataArtifact))
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(ctx)
		cs = l
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kw.Close()

	tabletMode := testParam.tablet
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
		if err = playstore.InstallApp(installCtx, a, d, pkgName, -1); err != nil {
			cancel()
			s.Fatalf("Failed to install %s: %v", pkgName, err)
		}
		cancel()
	}

	ac := uiauto.New(tconn)
	var pc pointer.Context
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
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller")
		}
		tsew, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to access to the touchscreen: ", err)
		}
		defer tsew.Close()
		stw, err := tsew.NewSingleTouchWriter()
		if err != nil {
			s.Fatal("Failed to create the single touch writer: ", err)
		}
		setOverviewModeAndWait = func(ctx context.Context) error {
			return ash.DragToShowOverview(ctx, tsew, stw, tconn)
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
				if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
					return errors.Wrap(err, "failed to show the hotseat")
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
				if err := pc.ClickAt(iconBounds[iconIdx].CenterPoint())(ctx); err != nil {
					return errors.Wrapf(err, "failed to click icon at %d", iconIdx)
				}
				if isPopup {
					menus := nodewith.ClassName("MenuItemView")
					if err := ac.WaitUntilExists(menus.First())(ctx); err != nil {
						return errors.Wrap(err, "failed to wait for the menu to appear")
					}
					menuInfo, err := ac.NodesInfo(ctx, menus)
					if err != nil {
						return errors.Wrap(err, "failed to find the menu items")
					}
					targetIdx := popupIdx + 1
					for i := 1; i < len(menuInfo); i++ {
						if hasYoutubeIcon && strings.HasPrefix(strings.ToLower(menuInfo[i].Name), "youtube") {
							targetIdx++
						}
					}
					if err := pc.Click(menus.Nth(targetIdx))(ctx); err != nil {
						return errors.Wrapf(err, "failed to click menu item %d", popupIdx)
					}
				}
				if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
					return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
				}); err != nil {
					return errors.Wrap(err, "failed to wait for the window animation")
				}
				return ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden)
			},
		}
	} else {
		pc = pointer.NewMouse(tconn)
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
	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(closeCtx)

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
			gotIt := d.Object(ui.Text("GOT IT"))
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
			pleaseAdd := d.Object(ui.Text("Please add at least one email address"))
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
			takeMe := d.Object(ui.Text("TAKE ME TO GMAIL"))
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
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			launchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if _, err := ash.GetARCAppWindowInfo(ctx, tconn, app.packageName); err == nil {
				testing.ContextLogf(ctx, "Package %s is already visible, skipping", app.packageName)
				return nil
			}
			return uiauto.Combine(
				"launch-app",
				openAppList,
				action.Sleep(time.Second),
				kw.TypeAction(app.query),
				// Finds the first search result tile and assume it is the app. Not
				// searching by name because launcher could append suffix to the app
				// name. E.g. gmail app has "Gmail, Installed app" as its name. The
				// suffix is for chromevox and could change.
				ac.WithTimeout(5*time.Second).LeftClick(nodewith.ClassName("SearchResultTileItemView").First()),
				func(ctx context.Context) error { return ash.WaitForVisible(ctx, tconn, app.packageName) },
				func(ctx context.Context) error {
					s.Log("Skipping the splash screen of ", app.query)
					return nil
				},
				app.skipSplash,
				// Waits some time to stabilize the result of launcher animations.
				action.Sleep(timeout),
			)(launchCtx)
		}); err != nil {
			s.Fatalf("Failed to launch %s: %v", app.query, err)
		}
	}

	// Whether to close "New Tab" window. Needed for lacros variation and need
	// to do the close after creating other lacros browser windows.
	needCloseNewTabWindow := testParam.useLacros

	// Here adds browser windows:
	// 1. webGL aquarium -- adding considerable load on graphics.
	// 2. chromium issue tracker -- considerable amount of elements.
	// 3. youtube -- the substitute of youtube app.
	for _, url := range []string{
		"https://webglsamples.org/aquarium/aquarium.html",
		"https://bugs.chromium.org/p/chromium/issues/list",
		"https://youtube.com/",
	} {
		conn, err := cs.NewConn(ctx, url, cdputil.WithNewWindow())
		if err != nil {
			s.Fatalf("Failed to open %s: %v", url, err)
		}
		// We don't need to keep the connection, so close it now.
		if err = conn.Close(); err != nil {
			s.Fatalf("Failed to close the connection to %s: %v", url, err)
		}

		// Lacros specific setup to close "New Tab" window.
		if needCloseNewTabWindow {
			w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
				return strings.HasPrefix(w.Title, "New Tab") && strings.HasPrefix(w.Name, "ExoShellSurface")
			})
			if err != nil {
				s.Fatal("Failed to find New Tab window: ", err)
			}
			if err := w.CloseWindow(ctx, tconn); err != nil {
				s.Fatal("Failed to close New Tab window: ", err)
			}
			needCloseNewTabWindow = false
		}

		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			if w.WindowType != ash.WindowTypeBrowser && w.WindowType != ash.WindowTypeLacros {
				return false
			}
			return !browserWindows[w.ID]
		})
		if err != nil {
			s.Fatalf("Failed to find the browser window for %s: %v", url, err)
		}
		browserWindows[w.ID] = true
		if !tabletMode {
			if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal); err != nil {
				s.Fatalf("Failed to change the window (%s) into the normal state: %v", url, err)
			}
		}
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
				if err := pc.ClickAt(targetWindow.OverviewInfo.Bounds.CenterPoint())(ctx); err != nil {
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
				if err := recorder.Run(ctx, func(ctx context.Context) error { return st.f(ctx, s, i) }); err != nil {
					s.Error("Failed to run the scenario: ", err)
				}
			}
		})
	}

	pv := perf.NewValues()
	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
