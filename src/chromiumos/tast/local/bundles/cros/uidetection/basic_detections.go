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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
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
		Fixture:      "chromeLoggedInClamshell",
		Timeout:      12 * time.Minute,
		Data:         []string{"logo_chrome.png"},
	})
}

func BasicDetections(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "basic_detections")

	ud := uidetection.NewDefault(tconn)
	ui := uiauto.New(tconn)

	chromeIcon := uidetection.CustomIcon(s.DataPath("logo_chrome.png"))
	addShortcut := uidetection.TextBlock([]string{"Add", "shortcut"})
	bottomBar := nodewith.ClassName("ShelfView")
	notificationArea := nodewith.ClassName("StatusAreaWidget")
	chromeWindow := nodewith.Role(role.Window).Name("Chrome - New Tab")

	verifyChromeIsMinimized := uiauto.NamedAction("verify that chrome is minimized",
		ui.WaitUntilExists(chromeWindow.Invisible()))
	verifyChromeIsShown := uiauto.NamedAction("verify that chrome is shown",
		ui.WaitUntilExists(chromeWindow.Visible()))

	expectError := func(f *uidetection.Finder, expectation string) func(ctx context.Context) error {
		return func(ctx context.Context) error {
			if loc, err := ud.Location(ctx, f); err == nil {
				return errors.Errorf("expected error: %s, actually successful with value %+v", expectation, loc)
			} else if !strings.Contains(err.Error(), expectation) {
				return errors.Errorf("expected error: %s, actual error: %s", expectation, err)
			}
			return nil
		}
	}

	maximizeButton := nodewith.Role(role.Button).ClassName("FrameCaptionButton").Name("Maximize")

	// Perform UI interaction to click Chrome logo to open Chrome,
	// click "Add shortcut", and click "cancel".
	// This covers all three UI detections: custom icon detection,
	// textblock detection and word detection.
	if err := uiauto.Combine("verify detections",
		uiauto.Combine("verify that basic matchers work",
			ud.LeftClick(chromeIcon),
			verifyChromeIsShown,
			ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(uidetection.TextBlock([]string{"Customize", "Chrome"})),
			ud.LeftClick(uidetection.Word("Cancel")),
			ud.WaitUntilGone(uidetection.Word("Cancel")),
			// Check the negative cases.
			expectError(uidetection.Word("Google"), uidetection.ErrMultipleMatch),
			expectError(uidetection.Word("Google").Nth(4), uidetection.ErrNthNotFound),
			expectError(uidetection.TextBlock([]string{"Add", "shortcut"}).Nth(2), uidetection.ErrNthNotFound),
			// Maximize chrome to ensure that the left and the right of the chrome
			// are empty bounding boxes later on. Do this early so that we don't
			// have a race condition later.
			ui.IfSuccessThen(ui.Exists(maximizeButton), ui.LeftClick(maximizeButton))),

		uiauto.Combine("verify that within successfully matches",
			uiauto.Combine("for px",
				expectError(chromeIcon.WithinPx(coords.NewRect(0, 0, 50, 50)), uidetection.ErrNotFound),
				ud.LeftClick(chromeIcon.WithinPx(coords.NewRect(50, 50, 9999, 9999))),
				verifyChromeIsMinimized),
			uiauto.Combine("for dp",
				expectError(chromeIcon.WithinDp(coords.NewRect(0, 0, 50, 50)), uidetection.ErrNotFound),
				ud.LeftClick(chromeIcon.WithinDp(coords.NewRect(50, 50, 9999, 9999))),
				verifyChromeIsShown),
			uiauto.Combine("for ui detection",
				expectError(chromeIcon.Within(addShortcut), uidetection.ErrNotFound),
				ud.LeftClick(chromeIcon.Within(chromeIcon)),
				verifyChromeIsMinimized),
			uiauto.Combine("for a11y nodes",
				expectError(chromeIcon.WithinA11yNode(notificationArea), uidetection.ErrNotFound),
				ud.LeftClick(chromeIcon.WithinA11yNode(bottomBar)),
				verifyChromeIsShown)),

		uiauto.Combine("verify that relative pixel successfully match",
			uiauto.Combine("for a11y nodes",
				expectError(chromeIcon.RightOfA11yNode(chromeWindow), uidetection.ErrEmptyBoundingBox),
				expectError(chromeIcon.LeftOfA11yNode(chromeWindow), uidetection.ErrEmptyBoundingBox),
				ud.LeftClick(chromeIcon.LeftOfA11yNode(notificationArea)),
				verifyChromeIsMinimized),
			uiauto.Combine("for dp",
				expectError(chromeIcon.LeftOfDp(50), uidetection.ErrNotFound),
				ud.LeftClick(chromeIcon.RightOfDp(50)),
				verifyChromeIsShown),
			uiauto.Combine("for ui detection",
				expectError(chromeIcon.Above(addShortcut), uidetection.ErrNotFound),
				ud.LeftClick(chromeIcon.Below(addShortcut)),
				verifyChromeIsMinimized),
			uiauto.Combine("for px",
				expectError(chromeIcon.AbovePx(100), uidetection.ErrNotFound),
				ud.LeftClick(chromeIcon.BelowPx(100)),
				verifyChromeIsShown)))(ctx); err != nil {
		s.Fatal("Failed to perform image-based UI interactions: ", err)
	}
}
