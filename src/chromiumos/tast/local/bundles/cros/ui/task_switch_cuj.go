// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"mute",
			"ui.TaskSwitchCUJ.username",
			"ui.TaskSwitchCUJ.password",
		},
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

	username := s.RequiredVar("ui.TaskSwitchCUJ.username")
	password := s.RequiredVar("ui.TaskSwitchCUJ.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(username, password, "gaia-id"),
		chrome.ARCSupported(), chrome.ExtraArgs("--arc-disable-app-sync",
			"--arc-disable-play-auto-install", "--arc-disable-locale-sync",
			"--arc-play-store-auto-update=off"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

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

	a, d, err := func() (*arc.ARC, *ui.Device, error) {
		arcSetupCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		// Optin to Play Store.
		s.Log("Opting into Play Store")
		if err := optin.Perform(arcSetupCtx, cr, tconn); err != nil {
			return nil, nil, errors.Wrap(err, "failed to optin to Play Store")
		}
		s.Log("Waiting for Playstore shown")
		if err := ash.WaitForVisible(arcSetupCtx, tconn, playStorePackageName); err != nil {
			return nil, nil, errors.Wrap(err, "failed to wait for the playstore")
		}

		a, err := arc.New(arcSetupCtx, s.OutDir())
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to start ARC")
		}
		// ui.NewDevice's context should be the one for the full test; otherwise
		// context expiry loses the connection to the UI Automator.
		d, err := ui.NewDevice(ctx, a)
		if err != nil {
			a.Close()
			return nil, nil, errors.Wrap(err, "failed to initialize UI Automator")
		}
		return a, d, nil
	}()
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer a.Close()
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
	}
	defer pc.Close()

	// Set up the cuj.Recorder: this test will measure the combinations of
	// animation smoothness for window-cycles (alt-tab selection), launcher,
	// and overview.
	configs := []cuj.MetricConfig{
		cuj.NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Container"),
		cuj.NewLatencyMetricConfig("Ash.DragWindowFromShelf.PresentationTime"),
		cuj.NewSmoothnessMetricConfig("Ash.SwipeHomeToOverviewGesture"),
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
		{"gmail", gmailPackageName, func(ctx context.Context) error {
			const (
				dialogID            = "com.google.android.gm:id/customPanel"
				dismissID           = "com.google.android.gm:id/gm_dismiss_button"
				customPanelMaxCount = 10
			)
			gotIt := d.Object(ui.TextMatches("GOT IT"))
			if err := gotIt.WaitForExists(ctx, timeout); err != nil {
				return errors.Wrap(err, `failed to find "GOT IT" button`)
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
	browserWindows := map[int]bool{}
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

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the list of windows: ", err)
	}
	s.Log("Switching the focused window through the overview mode")
	for i := 0; i < len(ws); i++ {
		if err := recorder.Run(ctx, tconn, func() error {
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
		}); err != nil {
			s.Fatal("Failed to run overview task switching: ", err)
		}
	}

	s.Log("Switching the focused window through alt-tab")
	for i := 0; i < len(ws); i++ {
		if err := recorder.Run(ctx, tconn, func() error {
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
		}); err != nil {
			s.Fatal("Failed to run alt-tab task switching: ", err)
		}
	}

	pv := perf.NewValues()
	if err = recorder.Record(pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
