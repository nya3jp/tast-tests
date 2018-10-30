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
		Timeout:      2 * time.Minute,
	})
}

func enableAccessibility(ctx context.Context, conn *chrome.Conn) error {
	err := conn.Exec(ctx, `
	window.__spoken_feedback_set_complete = false;
	chrome.accessibilityFeatures.spokenFeedback.set({value: true});
	chrome.accessibilityFeatures.spokenFeedback.get({}, () => {window.__spoken_feedback_set_complete = true;})`)
	if err != nil {
		return err
	}
	return nil
}

func isAccessibilityEnabled(ctx context.Context, a *arc.ARC) (bool, error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	if strings.TrimSpace(string(res)) == "1" {
		return true, nil
	}
	return false, nil
}

func SettingsBridge(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs([]string{"--force-renderer-accessibility"}))
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
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	err = testing.Poll(ctx, func(ctx context.Context) error {
		res, err := isAccessibilityEnabled(ctx, a)
		if err != nil {
			s.Fatal("isAccessibilityEnabled failed: ", err)
		}
		if !res {
			return nil
		}
		return errors.New("Timed out while waiting")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second})

	if err != nil {
		s.Fatal("Failed to check isAccessibilityEnabled: ", err)
	}

	if err = enableAccessibility(ctx, tconn); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	err = testing.Poll(ctx, func(ctx context.Context) error {
		res, err := isAccessibilityEnabled(ctx, a)
		if err != nil {
			s.Fatal("isAccessibilityEnabled failed: ", err)
		}
		if res {
			return nil
		}
		return errors.New("Timed out while waiting for for isAccessibilityEnabled")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second})

	if err != nil {
		s.Fatal("Failed to check isAccessibilityEnabled: ", err)
	}
}
