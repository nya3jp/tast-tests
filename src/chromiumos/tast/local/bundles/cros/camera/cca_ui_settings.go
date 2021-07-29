// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISettings,
		Desc:         "Opens CCA and verifies the settings menu behavior",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUISettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	subTestTimeout := 30 * time.Second
	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, *cca.App) error
	}{{
		"testFeedback",
		testFeedback,
	}, {
		"testHelp",
		testHelp,
	}, {
		"testGrid",
		testGrid,
	}, {
		"testTimer",
		testTimer,
	}} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			if err := cca.ClearSavedDirs(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			if err := cca.MainMenu.Open(ctx, app); err != nil {
				s.Fatal("Failed to click settings button: ", err)
			}
			defer cca.MainMenu.Close(cleanupCtx, app)

			if err := tst.testFunc(ctx, cr, app); err != nil {
				s.Fatalf("Failed to run subtest: %v: %v", tst.name, err)
			}

			// Restart app using non-shorten context.
			if err := app.Restart(cleanupCtx, tb); err != nil {
				s.Fatal("Failed to restart CCA: ", err)
			}
		})
		cancel()
	}
}

// testFeedback checks feedback button functionality.
func testFeedback(ctx context.Context, cr *chrome.Chrome, app *cca.App) error {
	if err := app.Click(ctx, cca.FeedbackButton); err != nil {
		return errors.Wrap(err, "failed to click feedback button")
	}
	matcher := func(t *target.Info) bool {
		return strings.Contains(t.URL, "gfdkimpbcpahaombhbimeihdjnejgicl") && t.Type == "app"
	}
	fConn, err := cr.NewConnForTarget(ctx, matcher)
	if err != nil {
		return errors.Wrap(err, "failed to open feedback app")
	}
	fConn.Close()
	return nil
}

// testHelp checks help button functionality.
func testHelp(ctx context.Context, cr *chrome.Chrome, app *cca.App) error {
	if err := app.Click(ctx, cca.HelpButton); err != nil {
		return errors.Wrap(err, "failed to click help button")
	}
	matcher := func(t *target.Info) bool {
		return strings.Contains(t.URL, "support.google.com") && t.Type == "page"
	}
	hConn, err := cr.NewConnForTarget(ctx, matcher)
	if err != nil {
		return errors.Wrap(err, "failed to open help app")
	}
	hConn.Close()
	return nil
}

// testGrid checks that changing grid type in settings is effective.
func testGrid(ctx context.Context, cr *chrome.Chrome, app *cca.App) error {
	if err := cca.GridTypeMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.GridTypeMenu.Close(ctx, app)

	if err := app.Click(ctx, cca.GoldenGridButton); err != nil {
		return errors.Wrap(err, "failed to click golden-grid button")
	}
	if err := app.WaitForState(ctx, "grid-golden", true); err != nil {
		return errors.Wrap(err, "failed to wait for golden-grid type being active")
	}
	return nil
}

// testTimer checks that changing timer duration in settings is effective.
func testTimer(ctx context.Context, cr *chrome.Chrome, app *cca.App) error {
	if err := cca.TimerMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.TimerMenu.Close(ctx, app)

	if err := app.Click(ctx, cca.Timer10sButton); err != nil {
		return errors.Wrap(err, "failed to click 10s-timer button")
	}
	if err := app.WaitForState(ctx, "timer-10s", true); err != nil {
		return errors.Wrap(err, "failed to wait for 10s-timer being active")
	}
	return nil
}
