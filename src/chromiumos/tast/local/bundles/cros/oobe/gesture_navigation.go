// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GestureNavigation,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test whether we show gesture navigation screens for a new users",
		Contacts: []string{
			"bohdanty@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
			"cros-oobe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.LoginTimeout + 3*time.Minute,
	})
}

func GestureNavigation(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "testpass"
	)
	var (
		getStarted = nodewith.Name("Get started").Role(role.Button)
		next       = nodewith.Name("Next").Role(role.Button)
	)

	cr, err := chrome.New(ctx,
		chrome.DontSkipOOBEAfterLogin(),
		chrome.FakeLogin(chrome.Creds{User: username, Pass: password}),
		chrome.ExtraArgs("--force-tablet-mode=touch_view"), // Tablet mode is needed to trigger gesture screens
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	func() {
		oobeConn, err := cr.WaitForOOBEConnection(ctx)
		if err != nil {
			s.Fatal("Failed to create OOBE connection: ", err)
		}
		defer oobeConn.Close()

		if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('gesture-navigation')", nil); err != nil {
			s.Fatal("Failed to advance to the gesture navigation screen: ", err)
		}

		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.GestureNavigation.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the gesture navigation screen: ", err)
		}
	}()

	ui := uiauto.New(tconn).WithTimeout(60 * time.Second)

	// Gesture navigation flow.
	if err := ui.WaitUntilExists(getStarted)(ctx); err != nil {
		s.Fatal("Failed to wait until gesture navigation main screen shown: ", err)
	}
	if err := ui.LeftClick(getStarted)(ctx); err != nil {
		s.Fatal("Failed to click on get started: ", err)
	}
	if err := ui.WaitUntilExists(next)(ctx); err != nil {
		s.Fatal("Failed to wait until go home shown: ", err)
	}
	if err := ui.LeftClick(next)(ctx); err != nil {
		s.Fatal("Failed to click on next: ", err)
	}
	if err := ui.WaitUntilExists(next)(ctx); err != nil {
		s.Fatal("Failed to wait until swotch to another app shown: ", err)
	}
	if err := ui.LeftClick(next)(ctx); err != nil {
		s.Fatal("Failed to click on next: ", err)
	}
	if err := ui.WaitUntilExists(next)(ctx); err != nil {
		s.Fatal("Failed to wait until go back shown: ", err)
	}
	if err := ui.LeftClick(next)(ctx); err != nil {
		s.Fatal("Failed to click on next: ", err)
	}
}
