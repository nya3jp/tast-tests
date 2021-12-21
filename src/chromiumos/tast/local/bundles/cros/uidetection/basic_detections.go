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

func BasicDetections(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "basic_detections")

	ud := uidetection.NewDefault(tconn)

	checkMultipleMatchError := func(ctx context.Context) error {
		if _, err := ud.Location(ctx, uidetection.Word("Google")); err == nil {
			return errors.Errorf("error %s is expected", uidetection.ErrMultipleMatch)
		} else if strings.Contains(err.Error(), uidetection.ErrMultipleMatch) {
			return nil
		} else {
			return errors.Errorf("expected error: %s, actual error: %s", uidetection.ErrMultipleMatch, err)
		}
	}
	// Perform UI interaction to click Chrome logo to open Chrome,
	// click "Add shortcut", and click "cancel".
	// This covers all three UI detections: custom icon detection,
	// textblock detection and word detection.
	if err := uiauto.Combine("verify detections",
		ud.LeftClick(uidetection.CustomIcon(s.DataPath("logo_chrome.png"))),
		ud.LeftClick(uidetection.TextBlock([]string{"Add", "shortcut"})),
		ud.LeftClick(uidetection.Word("Cancel")),
		ud.WaitUntilGone(uidetection.Word("Cancel")),
		checkMultipleMatchError,
	)(ctx); err != nil {
		s.Fatal("Failed to perform image-based UI interactions: ", err)
	}
}
