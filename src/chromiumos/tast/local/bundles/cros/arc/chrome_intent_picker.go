// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	arcui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeIntentPicker,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify Chrome Intent Picker can launch ARC app by visiting URL",
		Contacts: []string{
			"mxcai@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 10 * time.Minute,
	})
}

const (
	arcChromeIntentPickerUITimeout    = 15 * time.Second
	arcChromeIntentPickerPollInterval = 2 * time.Second
)

func ChromeIntentPicker(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	arcDevice := s.FixtValue().(*arc.PreData).ARC
	uiAutomator := s.FixtValue().(*arc.PreData).UIDevice

	const (
		appName        = "Intent Picker Test App"
		intentActionID = "org.chromium.arc.testapp.chromeintentpicker:id/intent_action"
		expectedAction = "android.intent.action.VIEW"
	)

	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup Test API Connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Setup ARC and Installs APK.
	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed waiting for intent helper: ", err)
	}

	if err := arcDevice.Install(ctx, arc.APKPath("ArcChromeIntentPickerTest.apk")); err != nil {
		s.Fatal("Failed installing the APK: ", err)
	}

	// Navigate to URL which ArcChromeIntentPickerTest app has associated an intent.
	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create renderer: ", err)
	}
	defer conn.Close()

	// Locate and left click on the Intent Picker button in Chrome omnibox.
	intentPicker := nodewith.ClassName("IntentPickerView").Role(role.Button)
	appLabel := nodewith.Name(appName).Role(role.Button)
	openButton := nodewith.Name("Open").Role(role.Button)
	ui := uiauto.New(tconn).WithInterval(arcChromeIntentPickerPollInterval)
	if err := uiauto.Combine("",
		ui.LeftClick(intentPicker),
		ui.LeftClick(appLabel),
		ui.LeftClick(openButton))(ctx); err != nil {
		s.Fatal("Failed to click intent picker button: ", err)
	}

	// Wait for the android intent to show in the Android test app.
	intentActionField := uiAutomator.Object(arcui.ID(intentActionID), arcui.Text(expectedAction))
	if err := intentActionField.WaitForExists(ctx, arcChromeIntentPickerUITimeout); err != nil {
		s.Fatalf("Failed waiting for intent action %q to appear in ARC window: %v", expectedAction, err)
	}
}
