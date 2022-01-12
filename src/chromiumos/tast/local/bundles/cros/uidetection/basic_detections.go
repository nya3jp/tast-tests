// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicDetections,
		Desc:         "Confirm that the image-based uidetetion library works as intended",
		Contacts:     []string{"alvinjia@google.com", "chromeos-engprod-sydney@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
		Data:         []string{"logo_chrome.png"},
	})
}

// expectError returns an action that checks if an expected error is returned by an action.
func expectError(action uiauto.Action, expectation string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := action(ctx); err == nil {
			return errors.Errorf("didn't get the expected error: %s", expectation)
		} else if !strings.Contains(err.Error(), expectation) {
			return errors.Errorf("expected error: %s, actual error: %s", expectation, err)
		}
		return nil
	}
}

func BasicDetections(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "basic_detections")

	ud := uidetection.NewDefault(tconn)
	// Perform UI interaction to click Chrome logo to open Chrome,
	// click "Add shortcut", and click "cancel".
	// This covers all three UI detections: custom icon detection,
	// textblock detection and word detection.
	if err := uiauto.Combine("verify detections",
		ud.LeftClick(uidetection.CustomIcon(s.DataPath("logo_chrome.png"))),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(uidetection.TextBlock([]string{"Customize", "Chrome"})),
		ud.LeftClick(uidetection.Word("Cancel")),
		ud.WaitUntilGone(uidetection.Word("Cancel")),
		// Check the negative cases.
		expectError(ud.Exists(uidetection.Word("Google")), uidetection.ErrMultipleMatch),
		expectError(ud.Exists(uidetection.Word("Google").Nth(4)), uidetection.ErrNthNotFound),
		expectError(ud.Exists(uidetection.TextBlock([]string{"Add", "shortcut"}).Nth(2)), uidetection.ErrNthNotFound),
	)(ctx); err != nil {
		s.Fatal("Failed to perform image-based UI interactions: ", err)
	}
}
