// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsBridge,
		Desc:         "Checks that Chrome settings are persisted in ARC",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Timeout:      1 * time.Minute,
	})
}

func EnableAccessibility(ctx context.Context, conn *chrome.Conn) error {
	err := conn.Exec(ctx, "window.__spoken_feedback_set_complete = false;chrome.accessibilityFeatures.spokenFeedback.set({value: true});  chrome.accessibilityFeatures.spokenFeedback.get({},function(d) {window.__spoken_feedback_set_complete = true;})")
	if err != nil {
		return errors.Wrap(err, "failed executing console.log")
	}
	return nil
}

func checkSettings(ctx context.Context, a *arc.ARC) (bool, error) {
	cmd := a.SendShellCommand(ctx, "settings --user 0 get secure accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	if strings.TrimRight(string(res), "\n") == "1" {
		return true, nil
	}
	return false, nil
}

func SettingsBridge(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.AccessibilityEnabled())
	if err != nil {
		s.Log("Failed to connect to Chrome: ", err)

	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC:", err)
	}
	defer a.Close()

	if err = a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}
	res, err := checkSettings(ctx, a)
	if err != nil {
		s.Fatal("Error with checkSettings:", err)
	}
	if res != false {
		s.Fatal("Accessibility was unexpectedly already enabled in Android at beginning of test.")
	}

	if err = EnableAccessibility(ctx, tconn); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	res, err = checkSettings(ctx, a)
	if err != nil {
		s.Fatal("Error with checkSettings: ", err)
	}
	if res != true {
		s.Fatal("AccessibilityEnabled not set to correct value in Android.")
	}
}
