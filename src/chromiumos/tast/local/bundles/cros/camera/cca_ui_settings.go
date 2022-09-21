// Copyright 2019 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISettings,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Opens CCA and verifies the settings menu behavior",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Params: []testing.Param{{
			Fixture: "ccaTestBridgeReady",
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "ccaTestBridgeReadyLacros",
		}},
	})
}

func CCAUISettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cca.FixtureData).Chrome
	bt := s.FixtValue().(cca.FixtureData).BrowserType
	runTestWithApp := s.FixtValue().(cca.FixtureData).RunTestWithApp

	subTestTimeout := 40 * time.Second
	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, browser.Type, *cca.App) error
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
			if err := runTestWithApp(ctx, func(ctx context.Context, app *cca.App) error {
				cleanupCtx := ctx
				ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
				defer cancel()

				if err := cca.MainMenu.Open(ctx, app); err != nil {
					s.Fatal("Failed to click settings button: ", err)
				}
				defer cca.MainMenu.Close(cleanupCtx, app)

				return tst.testFunc(ctx, cr, bt, app)
			}, cca.TestWithAppParams{}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tst.name, err)
			}
		})
		cancel()
	}
}

// testFeedback checks feedback button functionality.
func testFeedback(ctx context.Context, cr *chrome.Chrome, _ browser.Type, app *cca.App) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := app.Click(ctx, cca.FeedbackButton); err != nil {
		return errors.Wrap(err, "failed to click feedback button")
	}
	matcher := func(t *target.Info) bool {
		return strings.Contains(t.URL, "chrome://feedback")
	}
	fConn, err := cr.NewConnForTarget(ctx, matcher)
	if err != nil {
		return errors.Wrap(err, "failed to open feedback app")
	}
	defer fConn.Close()
	defer func(ctx context.Context) {
		if err := fConn.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close the feedback page")
		}
	}(cleanupCtx)
	return nil
}

// testHelp checks help button functionality.
func testHelp(ctx context.Context, cr *chrome.Chrome, bt browser.Type, app *cca.App) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := app.Click(ctx, cca.HelpButton); err != nil {
		return errors.Wrap(err, "failed to click help button")
	}
	br, brCleanUp, err := browserfixt.Connect(ctx, cr, bt)
	if err != nil {
		return errors.Wrap(err, "failed to connect to browser")
	}
	defer brCleanUp(ctx)
	matcher := func(t *target.Info) bool {
		return strings.Contains(t.URL, "support.google.com") && t.Type == "page"
	}
	hConn, err := br.NewConnForTarget(ctx, matcher)
	if err != nil {
		return errors.Wrap(err, "failed to open help app")
	}
	defer hConn.Close()
	defer func(ctx context.Context) {
		if err := hConn.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close the help page")
		}
	}(cleanupCtx)
	return nil
}

// testGrid checks that changing grid type in settings is effective.
func testGrid(ctx context.Context, cr *chrome.Chrome, _ browser.Type, app *cca.App) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// TODO(b/215484798): Removed the logic for old UI once the new UI applied.
	useOldUI, err := app.OptionExist(ctx, cca.GridOption)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of the grid toggle")
	}
	if !useOldUI {
		// The grid test for the new UI is moved to CCAUIPreviewOptions.
		return nil
	}

	if err := cca.GridTypeMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.GridTypeMenu.Close(cleanupCtx, app)

	if err := app.Click(ctx, cca.GoldenGridButton); err != nil {
		return errors.Wrap(err, "failed to click golden-grid button")
	}
	if err := app.WaitForState(ctx, "grid-golden", true); err != nil {
		return errors.Wrap(err, "failed to wait for golden-grid type being active")
	}
	return nil
}

// testTimer checks that changing timer duration in settings is effective.
func testTimer(ctx context.Context, cr *chrome.Chrome, _ browser.Type, app *cca.App) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// TODO(b/215484798): Removed the logic for old UI once the new UI applied.
	useOldUI, err := app.OptionExist(ctx, cca.TimerOption)
	if err != nil {
		return errors.Wrap(err, "failed to check the existence of the timer toggle")
	}
	if !useOldUI {
		// The timer test for the new UI is moved to CCAUIPreviewOptions.
		return nil
	}

	if err := cca.TimerMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.TimerMenu.Close(cleanupCtx, app)

	if err := app.Click(ctx, cca.Timer10sButton); err != nil {
		return errors.Wrap(err, "failed to click 10s-timer button")
	}
	if err := app.WaitForState(ctx, "timer-10s", true); err != nil {
		return errors.Wrap(err, "failed to wait for 10s-timer being active")
	}
	return nil
}
