// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Debug,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Demonstrates a problem getting bounds of a Gmail thread in Lacros",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// reportGmailThreadBounds gets the Gmail thread bounds, logs them, and shows them
// by moving the mouse along the boundary (for someone looking at the DUT display).
func reportGmailThreadBounds(ctx context.Context, tconn *chrome.TestConn) error {
	ac := uiauto.New(tconn)
	emailThreadBounds, err := ac.Location(ctx, nodewith.Role("genericContainer").ClassName("AO"))
	if err != nil {
		return errors.Wrap(err, "failed to get the Gmail thread location")
	}

	testing.ContextLog(ctx, emailThreadBounds)

	if err := action.Combine(
		"move the mouse along the boundary of the Gmail thread",
		mouse.Move(tconn, emailThreadBounds.TopLeft(), time.Second),
		ac.Sleep(time.Second),
		mouse.Move(tconn, emailThreadBounds.BottomLeft(), 2*time.Second),
		mouse.Move(tconn, emailThreadBounds.BottomRight(), 2*time.Second),
		mouse.Move(tconn, emailThreadBounds.TopRight(), 2*time.Second),
		mouse.Move(tconn, emailThreadBounds.TopLeft(), 2*time.Second),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to show the bounds of the Gmail thread")
	}

	return nil
}

func Debug(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)

	var cr *chrome.Chrome
	var cs ash.ConnSource
	switch bt {
	case browser.TypeAsh:
		cr = s.FixtValue().(cuj.FixtureData).Chrome
		cs = cr
	case browser.TypeLacros:
		var l *lacros.Lacros
		var err error
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros)
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		defer lacros.CloseLacros(cleanupCtx, l)
	default:
		s.Fatal("Unrecognized browser type: ", bt)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	conn, err := cs.NewConn(ctx, "https://www.gmail.com/")
	if err != nil {
		s.Fatal("Failed to navigate to Gmail: ", err)
	}
	defer conn.Close()

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get windows: ", err)
	}

	if wsCount := len(ws); wsCount != 1 {
		s.Fatal("Expected 1 window; found ", wsCount)
	}

	wID := ws[0].ID
	if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to normal: ", err)
	}

	if err := uiauto.New(tconn).LeftClick(nodewith.Role(role.Row).First())(ctx); err != nil {
		s.Fatal("Failed to click the Gmail thread row: ", err)
	}

	if err := webutil.WaitForQuiescence(ctx, conn, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for Gmail to finish loading: ", err)
	}

	if err := reportGmailThreadBounds(ctx, tconn); err != nil {
		s.Fatal("Failed to report the Gmail thread bounds before maximizing the window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set window state to maximized: ", err)
	}

	if err := reportGmailThreadBounds(ctx, tconn); err != nil {
		s.Fatal("Failed to report the Gmail thread bounds after maximizing the window: ", err)
	}
}
