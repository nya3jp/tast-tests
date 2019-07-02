// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	// TODO(crbug.com/963772): Move libraries in video to camera or media folder.
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISettings,
		Desc:         "Opens CCA and verifies the settings menu behavior",
		Contacts:     []string{"shenghao@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUISettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{
		s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching app: ", err)
	}
	s.Log("Preview started")

	if err := app.ClickWithSelector(ctx, "#open-settings"); err != nil {
		s.Fatal("Failed to click settings button: ", err)
	}

	// Check feedback button functionality.
	if err := app.ClickWithSelector(ctx, "#settings-feedback"); err != nil {
		s.Error("failed to click feedback button")
	}
	matcher := func(t *target.Info) bool { return t.Title == "Feedback" && t.Type == "app" }
	if err := cr.WaitForTarget(ctx, matcher, 10*time.Second); err != nil {
		s.Error("feedback app does not open")
	}

	// Check help button functionality.
	if err := app.ClickWithSelector(ctx, "#settings-help"); err != nil {
		s.Error("failed to click help button")
	}
	matcher = func(t *target.Info) bool {
		return strings.Contains(t.URL, "support.google.com") && t.Type == "page"
	}
	if err := cr.WaitForTarget(ctx, matcher, 10*time.Second); err != nil {
		s.Error("Help page does not open")
	}

	// Check that changing grid type in settings is effective.
	if err := app.ClickWithSelector(ctx, "#settings-gridtype"); err != nil {
		s.Error("failed to click grid type button: ", err)
	}
	if err := app.ClickWithSelector(ctx, "#grid-golden"); err != nil {
		s.Error("failed to click golden-grid button: ", err)
	}
	// Click back.
	if err := app.ClickWithSelector(ctx, "#gridsettings .menu-header button"); err != nil {
		s.Error("failed to click back button: ", err)
	}
	if err := app.WaitForState(ctx, "golden", true); err != nil {
		s.Error("golden-grid type is not active: ", err)
	}

	// Check that changing timer duration in settings is effective.
	if err := app.ClickWithSelector(ctx, "#settings-timerdur"); err != nil {
		s.Error("failed to click timer duration button: ", err)
	}
	if err := app.ClickWithSelector(ctx, "#timer-10s"); err != nil {
		s.Error("failed to click 10s-timer button: ", err)
	}
	// Click back.
	if err := app.ClickWithSelector(ctx, "#timersettings .menu-header button"); err != nil {
		s.Error("failed to click back button: ", err)
	}
	if err := app.WaitForState(ctx, "_10sec", true); err != nil {
		s.Error("10s-timer is not active: ", err)
	}
}
