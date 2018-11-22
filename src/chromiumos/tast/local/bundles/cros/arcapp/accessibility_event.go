// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"accessibility_sample.apk", "exepcted_output_accessibility_sample.txt"},
		Timeout:      4 * time.Minute,
	})
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		// This is a build of an application containing a single activity and basic UI elements..
		apk = "accessibility_sample.apk"
		pkg = "com.example.sarakato.accessibilitysample"
		cls = "com.example.sarakato.accessibilitysample.MainActivity"

		toggleButtonID = "com.example.sarakato.accessibilitysample:id/toggleButton"
		checkBoxID     = "com.example.sarakato.accessibilitysample:id/checkBox"
		cVoxExtID      = "mndnfokpggljbaajbnioimlmbfngpief"
		cVoxExtURL     = "/cvox2/background/background.html"

		accel = "Tab"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs([]string{"--force-renderer-accessibility"}))
	if err != nil {
		s.Log("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	// Run the sample application.
	// Waits for UI elements to appear before proceeding with enabling ChromeVox.
	apptest.RunWithChrome(ctx, s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		must(d.Object(ui.ID(toggleButtonID)).WaitForExists(ctx))
		must(d.Object(ui.ID(checkBoxID)).WaitForExists(ctx))
	})

	conn, err := cr.TestAPIConn(ctx)
	err = conn.Exec(ctx, `
		window.__spoken_feedback_set_complete = false;
		chrome.accessibilityFeatures.spokenFeedback.set({value: true});
		chrome.accessibilityFeatures.spokenFeedback.get({}, () => {
			window.__spoken_feedback_set_complete = true;
		});
	`)

	chromeVoxConn, err := cr.ExtConn(ctx, cVoxExtID, cVoxExtURL)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}

	ew, err := input.Keyboard(ctx)
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if err != nil {
		s.Fatalf("Error with creating EW from keyboard:", err)
	}

	if err := ew.Accel(ctx, accel); err != nil {
		s.Fatalf("Accel(%q) returned error: %v", accel, err)
	}

	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if err := ew.Accel(ctx, accel); err != nil {
		s.Fatalf("Accel(%q) returned error: %v", accel, err)
	}

	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if err := ew.Accel(ctx, accel); err != nil {
		s.Fatalf("Accel(%q) returned error: %v", accel, err)
	}

	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		s.Fatalf("Accel(%q) returned error: %v", accel, err)
	}

	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if err := ew.Accel(ctx, accel); err != nil {
		s.Fatalf("Accel(%q) returned error: %v", accel, err)
	}

	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		s.Fatalf("Accel(%q) returned error: %v", accel, err)
	}

	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	gotOutput := ""
	err = chromeVoxConn.EvalPromise(ctx, `
		new Promise((resolve, reject) => {
			var string = LogStore.instance.getLogs().toString();
			resolve(string);
		})
	`, &gotOutput)

	// Check ChromeVog log output matches with expected log.
	wantOutput, err := ioutil.ReadFile(s.DataPath("exepcted_output_accessibility_sample.txt"))
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	}

	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if strings.Compare(gotOutput, string(wantOutput)) != 0 {
		s.Fatalf("Output was not as expected")
	}
}
