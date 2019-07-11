// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityWebview,
		Desc:         "Checks ChromeVox functionality on a webview",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"ArcAccessibilityWebviewTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func AccessibilityWebview(ctx context.Context, s *testing.State) {
	const (
		// This is a build of an application containing a single activity and a webview element.
		// The source code is in vendor/google_arc.
		apkName      = "ArcAccessibilityWebviewTest.apk"
		webviewID    = "org.chromium.arc.testapp.accessibilitywebviewtest:id/webView"
		packageName  = "org.chromium.arc.testapp.accessibilitywebviewtest"
		activityName = "org.chromium.arc.testapp.accessibilitywebviewtest.MainActivity"
	)
	cr, err := accessibility.NewChrome(ctx)
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer cr.Close(ctx)

	a, err := accessibility.NewARC(ctx, s.OutDir())
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer a.Close()

	if err := accessibility.InstallAndStartApp(ctx, a, s.DataPath(apkName), packageName, activityName, []string{}); err != nil {
		s.Fatal("Setting up ARC environment with accessibility failed: ", err)
	}

	if err := accessibility.EnableSpokenFeedback(ctx, cr, a); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	chromeVoxConn, err := accessibility.ChromeVoxExtConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		s.Fatal("Could not wait for ChromeVox to stop speaking: ", err)
	}

	if err := accessibility.WaitForElementFocused(ctx, chromeVoxConn, "android.webkit.WebView"); err != nil {
		s.Fatal("Timed out polling for element: ", err)
	}
}
