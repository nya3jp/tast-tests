// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreview,
		Desc:         "Opens CCA and verifies the preview functions",
		Contacts:     []string{"inker@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Params: []testing.Param{{
			Pre: testutil.ChromeWithPlatformApp(),
			Val: testutil.PlatformApp,
		}, {
			Name: "swa",
			Pre:  testutil.ChromeWithSWA(),
			Val:  testutil.SWA,
		}},
	})
}

func CCAUIPreview(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	useSWA := s.Param().(testutil.CCAAppType) == testutil.SWA
	tb, err := testutil.NewTestBridge(ctx, cr, useSWA)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb, useSWA)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	restartApp := func() {
		s.Log("Restarts CCA")
		if err := app.Restart(ctx, tb, false); err != nil {
			var errJS *cca.ErrJS
			if errors.As(err, &errJS) {
				s.Error("There are JS errors when running CCA: ", err)
			} else {
				s.Fatal("Failed to restart CCA: ", err)
			}
		}
	}

	if err := testResize(ctx, app, useSWA); err != nil {
		s.Error("Failed in testResize(): ", err)
		restartApp()
	}

	// TODO(shik): Add the missing preview tests in go/cca-test:
	// * Preview active after going back from gallery
	// * Preview active after taking picture
	// * Preview active after recording
	// * Preview active after suspend/resume
}

func testResize(ctx context.Context, app *cca.App, useSWA bool) error {
	restore := func() error {
		if err := app.RestoreWindow(ctx); err != nil {
			return errors.Wrap(err, "failed to restore window")
		}
		// It is expected that the preview will only be active after the window
		// is focus on SWA.
		if useSWA {
			if err := app.Focus(ctx); err != nil {
				return errors.Wrap(err, "failed to focus window")
			}
		}
		if err := app.WaitForVideoActive(ctx); err != nil {
			return errors.Wrap(err, "preview is inactive after restoring window")
		}
		return nil
	}
	if err := restore(); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Maximizing window")
	if err := app.MaximizeWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to maximize window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after maximizing window")
	}
	if err := restore(); err != nil {
		return errors.Wrap(err, "failed in restore() after maximizing window")
	}

	testing.ContextLog(ctx, "Fullscreening window")
	if err := app.FullscreenWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to fullscreen window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after fullscreening window")
	}
	if err := restore(); err != nil {
		return errors.Wrap(err, "failed in restore() after fullscreening window")
	}

	testing.ContextLog(ctx, "Minimizing window")
	if err := app.MinimizeWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to minimize window")
	}
	if err := app.CheckVideoInactive(ctx); err != nil {
		return errors.Wrap(err, "preview is active after minimizing window")
	}
	if err := restore(); err != nil {
		return errors.Wrap(err, "failed in restore() after maximizing window")
	}

	return nil
}
