// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      20 * time.Minute,
		Vars: []string{
			"mute",
			"ui.TaskSwitchCUJ.username",
			"ui.TaskSwitchCUJ.password",
		},
	})
}

func TaskSwitchCUJ(ctx context.Context, s *testing.State) {
	const (
		searchIconID         = "com.android.vending:id/search_bar"
		playStorePackageName = "com.android.vending"
		gmailPackageName     = "com.google.android.gm"
		calendarPackageName  = "com.google.android.calendar"
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
	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode status: ", err)
	}

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	s.Log("Waiting for Playstore shown")
	if err := ash.WaitForVisible(ctx, tconn, playStorePackageName); err != nil {
		s.Fatal("Failed to wait for the playstore: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close()

	// Install android apps for the everyday works: Gmail and Google Calendar.
	// Skipping youtube app since there's a trouble of installing it.
	// TODO(mukai): fix the problem of installing youtube.
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("Failed to list the installed packages: ", err)
	}
	for _, pkgName := range []string{gmailPackageName, calendarPackageName} {
		if _, ok := pkgs[pkgName]; ok {
			s.Logf("%s is already installed", pkgName)
			continue
		}
		s.Log("Installing ", pkgName)
		if err = playstore.InstallApp(ctx, a, d, pkgName); err != nil {
			s.Fatalf("Failed to install %s: %v", pkgName, err)
		}
		// Go back until the search bar appears.
		searchIcon := d.Object(ui.ID(searchIconID))
		for {
			if err = kw.Accel(ctx, topRow.BrowserBack); err != nil {
				s.Fatal("Failed to press back: ", err)
			}
			if err := searchIcon.WaitForExists(ctx, timeout); err == nil {
				break
			}
		}
	}

	// Close the play-store app window through ctrl-w.
	if err := kw.Accel(ctx, "ctrl+w"); err != nil {
		s.Fatal("Failed to close the play store app: ", err)
	}

	if err = cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for idle-ness: ", err)
	}

	numWindows := 0

	// Set up the cuj.Recorder: this test will measure the combinations of
	// animation smoothness for window-cycles (alt-tab selection), launcher,
	// and overview.
	configs := []cuj.MetricConfig{{
		HistogramName: "Ash.WindowCycleView.AnimationSmoothness.Container",
		Unit:          "percent",
		Category:      cuj.CategorySmoothness,
	}}
	for _, state := range []string{"Peeking", "Close", "Half"} {
		configs = append(configs, cuj.MetricConfig{
			HistogramName: "Apps.StateTransition.AnimationSmoothness." + state + ".ClamshellMode",
			Unit:          "percent",
			Category:      cuj.CategorySmoothness,
		})
	}
	for _, suffix := range []string{"SingleClamshellMode", "ClamshellMode", "TabletMode"} {
		configs = append(configs, cuj.MetricConfig{
			HistogramName: "Ash.Overview.AnimationSmoothness.Enter." + suffix,
			Unit:          "percent",
			Category:      cuj.CategorySmoothness,
		}, cuj.MetricConfig{
			HistogramName: "Ash.Overview.AnimationSmoothness.Exit." + suffix,
			Unit:          "percent",
			Category:      cuj.CategorySmoothness,
		})
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
		skipSplash  func() error
	}{
		{"gmail", gmailPackageName, func() error {
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
		{"calendar", calendarPackageName, func() error {
			const (
				nextArrowID       = "com.google.android.calendar:id/next_arrow_touch"
				rightArrowID      = "com.google.android.calendar:id/right_arrow"
				permissionAllowID = "com.android.packageinstaller:id/permission_allow_button"
				maxPermissions    = 10
				maxSplashes       = 10
			)
			gotIt := d.Object(ui.TextMatches("GOT IT|Got it"))
			nextArrow := d.Object(ui.ID(nextArrowID))
			rightArrow := d.Object(ui.ID(rightArrowID))
			gotItFound := false
			for i := 0; i < maxSplashes; i++ {
				if err := gotIt.WaitForExists(ctx, timeout); err == nil {
					gotItFound = true
					break
				}
				nextButton := nextArrow
				if err := nextArrow.Exists(ctx); err != nil {
					if err := rightArrow.Exists(ctx); err != nil {
						return errors.Wrap(err, "no buttons exist to skip the splash screen")
					}
					nextButton = rightArrow
				}
				if err := nextButton.Click(ctx); err != nil {
					return errors.Wrap(err, "failed to click the button to skip splash screen")
				}
			}
			if !gotItFound {
				return errors.Wrap(err, `failed to click "GOT IT" button; too many splashes?`)
			}
			if err := gotIt.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click "GOT IT" button`)
			}
			// Google calendar will ask some permissions, here allows them by clicking
			// 'OK' button.
			permissionAllow := d.Object(ui.ID(permissionAllowID))
			for i := 0; i < maxPermissions; i++ {
				if err := permissionAllow.WaitForExists(ctx, timeout); err != nil {
					// No more permission dialogs.
					return nil
				}
				if err := permissionAllow.Click(ctx); err != nil {
					return errors.Wrap(err, "failed to click the permission allow button")
				}
			}
			return errors.New("too many permission dialogs")
		}},
	} {
		if err = recorder.Run(ctx, tconn, func() error {
			if err := kw.Accel(ctx, "search"); err != nil {
				return errors.Wrap(err, "failed to type the search key")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.Type(ctx, app.query); err != nil {
				return errors.Wrap(err, "failed to type the query")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.Accel(ctx, "enter"); err != nil {
				return errors.Wrap(err, "failed to type the enter key")
			}
			if err := ash.WaitForVisible(ctx, tconn, app.packageName); err != nil {
				return errors.Wrapf(err, "failed to wait for the new window of %s", app.packageName)
			}
			s.Log("Skipping the splash screen of ", app.query)
			if err = app.skipSplash(); err != nil {
				return errors.Wrap(err, "failed to skip the splash screen of the app")
			}
			numWindows++
			// Waits some time to stabilize the result of launcher animations.
			return testing.Sleep(ctx, timeout)
		}); err != nil {
			s.Fatalf("Failed to launch %s: %v", app.query, err)
		}
	}

	// Here adds browser windows:
	// 1. webGL aquarium -- adding considerable load on graphics.
	// 2. chromium issue tracker -- considerable amount of elements.
	// 3. youtube.com -- substitute of the youtube app.
	browserWindows := map[int]bool{}
	for _, url := range []string{
		"https://webglsamples.org/aquarium/aquarium.html",
		"https://bugs.chromium.org/p/chromium/issues/list",
		"https://youtube.com",
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
	numWindows += len(browserWindows)

	if err = cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for idle-ness: ", err)
	}

	s.Log("Switching the focused window through the overview mode")
	for i := 0; i < numWindows; i++ {
		if err := recorder.Run(ctx, tconn, func() error {
			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				return errors.Wrap(err, "failed to enter into the overview mode")
			}
			// Uses the arrow key to select the next focused window.
			// Assumption: the order of the window in the overview mode is LRU, so
			// pressing the right-arrow for the number of windows means to select
			// the last-active window.
			// Note that there is 'new desks' button in the clamshell mode which takes
			// the keyboard focus. Thus the number of right keys needs to be increased
			// by one.
			// TODO(mukai): find the way to check if the target window gets the
			// focus or not.
			hitCount := numWindows
			if !tabletMode {
				hitCount++
			}
			for j := 0; j < hitCount; j++ {
				if err := kw.Accel(ctx, "Right"); err != nil {
					return errors.Wrap(err, "failed to type the right key")
				}
			}
			if err := kw.Accel(ctx, "Enter"); err != nil {
				return errors.Wrap(err, "failed to type the enter key")
			}
			return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.OverviewInfo == nil && w.IsActive
			}, &testing.PollOptions{Timeout: timeout})
		}); err != nil {
			s.Fatal("Failed to run overview task switching: ", err)
		}
	}

	s.Log("Switching the focused window through alt-tab")
	for i := 0; i < numWindows; i++ {
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
			for j := 0; j < numWindows-1; j++ {
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
