// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelectKeyboardBacklight,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test selecting keyboard backlight color in personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model("vell")),
		Timeout:      3 * time.Minute,
		Fixture:      "personalizationWithRgbKeyboard",
	})
}

func SelectKeyboardBacklight(ctx context.Context, s *testing.State) {
	const (
		backlightColor1 = "Blue"
		backlightColor2 = "Rainbow"
	)
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine("open Personalization Hub and verify Keyboard settings available",
		personalization.OpenPersonalizationHub(ui),
		ui.WaitUntilExists(nodewith.Role(role.StaticText).NameContaining("Keyboard backlight")))(ctx); err != nil {
		s.Fatal("Failed to show Keyboard settings: ", err)
	}

	if err := testKeyboardBacklight(ui, backlightColor1)(ctx); err != nil {
		s.Fatalf("Failed to select backlight color %v: %v", backlightColor1, err)
	}

	if err := testKeyboardBacklight(ui, backlightColor2)(ctx); err != nil {
		s.Fatalf("Failed to select backlight color %v: %v", backlightColor2, err)
	}
}

func testKeyboardBacklight(ui *uiauto.Context, backlightColor string) uiauto.Action {
	colorOption := nodewith.HasClass("color-container").Name(backlightColor)
	selectedColor := nodewith.HasClass("color-container tast-selected-color").Name(backlightColor)

	return uiauto.Combine("validate the selected backlight color",
		ui.LeftClick(colorOption),
		ui.WaitUntilExists(selectedColor))
}
