// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/browser"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FocusAndroidApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that assistant focuses Android app if both web and Android versions are open",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantWithArc",
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func FocusAndroidApp(ctx context.Context, s *testing.State) {
	const (
		QueryOpenGoogleNews = "Open Google News"
	)

	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome
	a := fixtData.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := assistant.InstallTestApkAndWaitReady(ctx, tconn, a); err != nil {
		s.Fatal("Failed to install a test app: ", err)
	}

	if err := launcher.LaunchApp(tconn, assistant.GoogleNewsAppTitle)(ctx); err != nil {
		s.Fatal("Failed to launch Google News Android: ", err)
	}

	if err := assistant.WaitForGoogleNewsAppActivation(ctx, tconn); err != nil {
		s.Fatal("Failed to wait Google News Android gets active: ", err)
	}

	// TODO(b/245349115): Remove this work around once the bug gets fixed.
	// Maximize and un-maximize Google News Android app to work around Ash and Arc WM state
	// synchronization issue.
	if _, err := ash.SetARCAppWindowStateAndWait(
		ctx, tconn, assistant.GoogleNewsPackageName, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize Google News Android app window: ", err)
	}

	if _, err := ash.SetARCAppWindowStateAndWait(
		ctx, tconn, assistant.GoogleNewsPackageName, ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to un-maximize Google News Android app window: ", err)
	}

	if _, err = browser.Launch(ctx, tconn, cr, assistant.GoogleNewsWebURL); err != nil {
		s.Fatal("Failed to launch Google News Web: ", err)
	}

	if err := assistant.WaitForGoogleNewsWebActivation(ctx, tconn); err != nil {
		s.Fatal("Failed to wait Google News Web gets active: ", err)
	}

	if _, err := assistant.SendTextQuery(ctx, tconn, QueryOpenGoogleNews); err != nil {
		s.Fatal("Failed to send Assistant text query: ", err)
	}

	if err := assistant.WaitForGoogleNewsAppActivation(ctx, tconn); err != nil {
		s.Fatal("Failed to wait Google News Android gets active: ", err)
	}
}
