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
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 3*time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func GestureNavigation(ctx context.Context, s *testing.State) {
	var (
		acceptAndContinue = nodewith.Name("Accept and continue").Role(role.Button)
		getStarted        = nodewith.Name("Get started").Role(role.Button)
		next              = nodewith.Name("Next").Role(role.Button)
		noThanks          = nodewith.Name("No thanks").Role(role.Button)
		skip              = nodewith.Name("Skip").Role(role.StaticText)
	)

	cr, err := chrome.New(ctx,
		chrome.DontSkipOOBEAfterLogin(),
		chrome.DisableFeatures("OobeConsolidatedConsent", "EnableOobeThemeSelection"),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
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
	ui := uiauto.New(tconn).WithTimeout(60 * time.Second)

	// Accept terms of service.
	if err := ui.WaitUntilExists(acceptAndContinue)(ctx); err != nil {
		s.Fatal("Failed to wait until sync consent shown: ", err)
	}
	if err := ui.LeftClick(acceptAndContinue)(ctx); err != nil {
		s.Fatal("Failed to click on accept and continue: ", err)
	}
	// Skip fingerprint setup screen.
	if err := ui.WaitUntilExists(skip)(ctx); err != nil {
		s.Fatal("Failed to wait until fingerprint screen shown: ", err)
	}
	if err := ui.LeftClick(skip)(ctx); err != nil {
		s.Fatal("Failed to click on skip: ", err)
	}
	// Skip pin setup screen.
	if err := ui.WaitUntilExists(skip)(ctx); err != nil {
		s.Fatal("Failed to wait until pin setup shown: ", err)
	}
	if err := ui.LeftClick(skip)(ctx); err != nil {
		s.Fatal("Failed to click on skip: ", err)
	}
	// Skip assistant flow.
	if err := ui.WaitUntilExists(noThanks)(ctx); err != nil {
		s.Fatal("Failed to wait until hey google screen shown: ", err)
	}
	if err := ui.LeftClickUntil(noThanks, ui.Gone(noThanks))(ctx); err != nil {
		s.Fatal("Failed to click on no thanks: ", err)
	}

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
	// Pass marketing opt in screen.
	if err := ui.WaitUntilExists(getStarted)(ctx); err != nil {
		s.Fatal("Failed to wait until marketing opt in shown: ", err)
	}
	if err := ui.LeftClick(getStarted)(ctx); err != nil {
		s.Fatal("Failed to click on get started: ", err)
	}
}
