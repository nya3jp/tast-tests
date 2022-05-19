// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"time"

	arcpkg "chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DynamicColor,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ArcSystemUIService changes Settings.Secure",
		Contacts:     []string{"arc-app-dev@google.com, ttefera@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm_t"},
		Fixture:      "arcBootedWithoutUIAutomator",
		Timeout:      chrome.GAIALoginTimeout + arcpkg.BootTimeout + 120*time.Second,
	})
}

func DynamicColor(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arcpkg.PreData).Chrome
	arc := s.FixtValue().(*arcpkg.PreData).ARC

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

	cmd := arc.Command(ctx, "settings", "get", "secure", "theme_customization_overlay_packages")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get secure settings: ", err)
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(string(output)), &result)
	themeOverlay := result
	if palette := themeOverlay["android.theme.customization.system_palette"].(float64); palette != 50 {
		s.Error("System_palette wanted: 50 was: ", palette)
	}
	if style := themeOverlay["android.theme.customization.theme_style"].(string); style != "EXPRESSIVE" {
		s.Error("Theme_style wanted: EXPRESSIVE was: ", style)
	}
}
