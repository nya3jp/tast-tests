// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrivacyIndicators,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check if the privacy indicators view show up when entering Google Meet",
		Contacts:     []string{"leandre@chromium.org", "cros-status-area-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.PrivacyIndicators.meet_code",
		},
		Timeout: 3 * time.Minute,
		Fixture: "chromeLoggedInWithCalendarEvents",
	})
}

func PrivacyIndicators(ctx context.Context, s *testing.State) {
	// Shorten context to allow for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}
	ui := uiauto.New(tconn)
	meetingCode := s.RequiredVar("ui.PrivacyIndicators.meet_code")

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	meetConn, err := cr.NewConn(ctx, "https://meet.google.com/"+meetingCode, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer meetConn.Close()

	// Match window titles `Google Meet` and `meet.google.com`.
	meetRE := regexp.MustCompile(`\bMeet\b|\bmeet\.\b`)
	meetWindow, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool { return meetRE.MatchString(w.Title) })
	if err != nil {
		s.Fatal("Failed to find the Meet window: ", err)
	}

	inTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to detect it is in tablet-mode or not: ", err)
	}
	s.Logf("Is in tablet-mode: %t", inTabletMode)

	var pc pointer.Context
	if inTabletMode {
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
	} else {
		pc = pointer.NewMouse(tconn)
	}
	defer pc.Close()

	uiWait := ui.WithTimeout(10 * time.Second)
	bubble := nodewith.ClassName("PermissionPromptBubbleView").First()
	allow := nodewith.Name("Allow").Role(role.Button).Ancestor(bubble)

	// Check and grant permissions.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Long wait for permission bubble and break poll loop when it times out.
		if err := uiWait.WaitUntilExists(bubble)(ctx); err != nil {
			return nil
		}

		if err := pc.Click(allow)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the allow button")
		}

		return errors.New("granting permissions")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}); err != nil {
		s.Fatal("Failed to grant permissions: ", err)
	}

	if err := meetWindow.ActivateWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to activate the Meet window: ", err)
	}

	privacyIndicators := nodewith.ClassName("PrivacyIndicatorsTrayItemView").First()
	if err := uiWait.WaitUntilExists(privacyIndicators)(ctx); err != nil {
		s.Fatal("Privacy indicators view does not show up as expected: ", err)
	}

	// Close the Meet window, expect privacy indicators view to be gone.
	if err := meetWindow.CloseWindow(cleanupCtx, tconn); err != nil {
		s.Error("Failed to close the meeting: ", err)
	}
	if err := uiWait.WaitUntilGone(privacyIndicators)(ctx); err != nil {
		s.Fatal("Privacy indicators view does not disappear as expected: ", err)
	}
}
