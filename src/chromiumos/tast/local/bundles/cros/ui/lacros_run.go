// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosRun,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test lacros-Chrome metrics and fixtures",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Name:    "fake_login_lacros",
				Fixture: "lacrosPrimary",
				Val:     "fake_login_lacros",
			}, {
				Name:    "gaia_keep_state_lacros",
				Fixture: "loggedInAndKeepStateLacros",
				Val:     "gaia_keep_state_lacros",
			}, {
				Name:    "gaia_not_keep_state_lacros",
				Fixture: "loggedInToCUJUserLacros",
				Val:     "gaia_not_keep_state_lacros",
			}, {
				Name:    "gaia_keep_state_ash",
				Fixture: "loggedInAndKeepState",
				Val:     "gaia_keep_state_ash",
			}, {
				Name:    "gaia_not_keep_state_ash",
				Fixture: "loggedInToCUJUser",
				Val:     "gaia_not_keep_state_ash",
			},
		},
	})
}

func LacrosRun(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	_, l, _, err := lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to set up lacros test: ", err)
	}
	bTconn, err := l.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	// defer lacros.CloseLacros(ctx, l)
	if err := clickChromeFromShelf(ctx, tconn); err != nil {
		s.Fatal("Failed to click chrome from shelf: ", err)
	}

	ui := uiauto.New(tconn)

	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.NewPerformanceCUJOptions())
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(ctx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Navigate to google.com")
		_, err = l.NewConn(ctx, "https://www.google.com")
		if err != nil {
			s.Fatal("Failed to open new tab: ", err)
		}

		searchBox := nodewith.Name("Search").Role(role.TextFieldWithComboBox).Focused()
		testing.ContextLog(ctx, "Click search box")
		if err := ui.LeftClick(searchBox)(ctx); err != nil {
			s.Fatal("Failed to click search box: ", err)
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		defer kb.Close()
		testing.ContextLog(ctx, "Type text in search box")
		if err := kb.Type(ctx, "Hello"); err != nil {
			s.Fatal("Failed to enter search text Hello: ", err)
		}

		if err := kb.AccelAction("Enter")(ctx); err != nil {
			s.Fatal("Failed to typer enter to start search: ", err)
		}
		testing.Sleep(ctx, 5*time.Second)

		if err := launchHelpApp(ctx, tconn); err != nil {
			s.Fatal("Failed to launch Help app: ", err)
		}
		testing.Sleep(ctx, 5*time.Second)
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test: ", err)
	}

	pv := perf.NewValues()

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the performance data: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}

func clickChromeFromShelf(ctx context.Context, tconn *chrome.TestConn) error {
	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not find the Chrome app")
	}
	appName := chromeApp.Name
	appIcon := nodewith.ClassName("ash/ShelfAppButton").Name(appName)
	ui := uiauto.New(tconn)
	// Click mouse to launch app.
	if err := ui.LeftClick(appIcon)(ctx); err != nil {
		return errors.Wrapf(err, "failed to launch app %q", appName)
	}
	return nil
}

func launchHelpApp(ctx context.Context, tconn *chrome.TestConn) error {
	settings, err := ossettings.LaunchAtPage(ctx, tconn, ossettings.AboutChromeOS)
	if err != nil {
		return errors.Wrap(err, "failed to launch Settings")
	}

	if err := settings.LaunchHelpApp()(ctx); err != nil {
		return errors.Wrap(err, "failed to launch Help app")
	}
	return nil
}
