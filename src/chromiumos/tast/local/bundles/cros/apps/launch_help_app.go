// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	tabletMode bool
	oobe       bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpApp,
		Desc: "Help app should be launched after OOBE",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"apps.LaunchHelpApp.consumer_username", "apps.LaunchHelpApp.consumer_password"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testParameters{
				tabletMode: false,
				oobe:       true,
			},
		}, {
			Name: "tablet_oobe",
			Val: testParameters{
				tabletMode: true,
				oobe:       true,
			},
		}, {
			Name: "clamshell_logged_in",
			Val: testParameters{
				tabletMode: false,
				oobe:       false,
			},
			Pre: chrome.LoggedIn(),
		}, {
			Name: "tablet_logged_in",
			Val: testParameters{
				tabletMode: true,
				oobe:       false,
			},
			Pre: chrome.LoggedIn(),
		},
		}})
}

// LaunchHelpApp verifies launching Showoff after OOBE.
func LaunchHelpApp(ctx context.Context, s *testing.State) {
	if s.Param().(testParameters).oobe {
		helpAppLaunchDuringOOBE(ctx, s, s.Param().(testParameters).tabletMode)
	} else {
		helpAppLaunchAfterLogin(ctx, s, s.Param().(testParameters).tabletMode)
	}
}

// helpAppLaunchDuringOOBE verifies help app launch during OOBE stage. Help app only launches with real user login in clamshell mode.
func helpAppLaunchDuringOOBE(ctx context.Context, s *testing.State, isTabletMode bool) {
	username := s.RequiredVar("apps.LaunchHelpApp.consumer_username")
	password := s.RequiredVar("apps.LaunchHelpApp.consumer_password")

	extraArgs := ""
	if isTabletMode {
		extraArgs = "--force-tablet-mode=touch_view"
	} else {
		extraArgs = "--force-tablet-mode=clamshell"
	}
	cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin(), chrome.DontSkipOOBEAfterLogin(), chrome.ExtraArgs(extraArgs))

	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Verify HelpApp (aka Explore) launched in Clamshell mode only.
	if err := assertHelpAppLaunched(ctx, s, tconn, cr, !isTabletMode); err != nil {
		s.Fatalf("Failed to verify help app launching during oobe in tablet mode enabled(%v): %v", isTabletMode, err)
	}
}

// helpAppLaunchAfterLogin verifies help app launch after user login. It should be able to launch on devices in both clamshell and tablet mode.
func helpAppLaunchAfterLogin(ctx context.Context, s *testing.State, isTabletMode bool) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	s.Logf("Ensure tablet mode enabled(%v)", isTabletMode)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet mode enabled(%v): %v", isTabletMode, err)
	}
	defer cleanup(ctx)

	app := apps.Help
	s.Logf("Launching %s", app.Name)
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		s.Fatalf("Failed to launch %s: %v", app.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
	}

	if err := assertHelpAppLaunched(ctx, s, tconn, cr, true); err != nil {
		s.Fatal("Failed to verify help app launching after user logged in: ", err)
	}
}

// assertHelpAppLaunched asserts help app to be launched or not
func assertHelpAppLaunched(ctx context.Context, s *testing.State, tconn *chrome.TestConn, cr *chrome.Chrome, isLaunched bool) error {
	if isLaunched {
		helpAppNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: apps.Help.Name}, 20*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to launch help app")
		}

		// Collect loadTimeData once launched.
		if err := logRuntimeData(ctx, s, cr); err != nil {
			return errors.Wrap(err, "failed to collect loadTimeData")
		}

		// Find Overview tab to verify app rendering.
		params := ui.FindParams{
			Name: "Overview",
			Role: ui.RoleTypeTab,
		}
		if _, err := helpAppNode.DescendantWithTimeout(ctx, params, 20*time.Second); err != nil {
			return errors.Wrap(err, "failed to render help app")
		}
	} else {
		isHelpAppLaunched, err := ui.Exists(ctx, tconn, ui.FindParams{Name: apps.Help.Name})
		if err != nil {
			return errors.Wrap(err, "failed to check HelpApp existence")
		}

		if isHelpAppLaunched {
			return errors.New("Help app is launched in Tablet mode")
		}
	}
	return nil
}

// logRuntimeData logs the window.loadTimeData() info to a file.
func logRuntimeData(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {
	const (
		logFileName = "loadTimeData.json"
		helpAppURL  = "chrome-untrusted://help-app/app.html"
	)
	helpConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(helpAppURL))
	if err != nil {
		return errors.Wrap(err, "failed to connect to help app")
	}

	var loadTimeData interface{}
	if err := helpConn.Eval(ctx, "window.loadTimeData", &loadTimeData); err != nil {
		return err
	}

	file := filepath.Join(s.OutDir(), logFileName)
	testing.ContextLogf(ctx, "Write loadTimeData into file: %s", file)

	bContent, err := json.Marshal(loadTimeData)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, bContent, 0644)
}

// shouldLaunchHelp returns a result to launch help app or not.
func shouldLaunchHelp(isTabletMode, isOOBE bool) bool {
	return !isOOBE || !isTabletMode
}
