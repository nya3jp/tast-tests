// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DynamicColor,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ArcSystemUIService changes Settings.Secure",
		Contacts:     []string{"arc-app-dev@google.com, ttefera@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "hatch_arc_t"},
		Fixture:      "arcBootedWithoutUIAutomator",
	})
}

// DynamicColor calls function in ArcSystemUIService and checks to see if Android Settings.Secure changed.
func DynamicColor(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	var ret bool
	if err := tconn.Call(ctx, &ret, "tast.promisify(chrome.autotestPrivate.sendArcOverlayColor)", 50, standardizedtestutil.Expressive); err != nil {
		s.Fatal(err, " Failed to send overlay color.")
	}
	if !ret {
		s.Fatal("sendOverlayColor function failed")
	}

	a := s.FixtValue().(*arc.PreData).ARC
	cmd := a.Command(ctx, "settings", "get", "secure", "theme_customization_overlay_packages")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get secure settings: ", err)
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(string(output)), &result)
	themeOverlay := result
	if palette := themeOverlay["android.theme.customization.system_palette"].(float64); palette != 50 {
		s.Error("system_palette wanted: 50 was: ", palette)
	}
	if style := themeOverlay["android.theme.customization.theme_style"].(string); style != "EXPRESSIVE" {
		s.Error("theme_style wanted: EXPRESSIVE was: ", style)
	}
}
