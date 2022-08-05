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
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func GestureNavigation(ctx context.Context, s *testing.State) {
	var (
		acceptAndContinue = nodewith.Name("Accept and continue").Role(role.Button)
		assistantPage     = nodewith.ClassName("assistant-optin-flow")
		getStarted        = nodewith.Name("Get started").Role(role.Button)
		next              = nodewith.Name("Next").Role(role.Button)
		noThanks          = nodewith.Name("No thanks").Role(role.Button)
		skip              = nodewith.Name("Skip").Role(role.StaticText)
	)

	cr, err := chrome.New(ctx,
		chrome.DontSkipOOBEAfterLogin(),
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

	if err := uiauto.Combine("Go through the OOBE flow UI after the GAIA login",
		uiauto.IfSuccessThen(ui.WaitUntilExists(acceptAndContinue), ui.LeftClick(acceptAndContinue)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(skip), ui.LeftClick(skip)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(skip), ui.LeftClick(skip)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(noThanks), ui.LeftClick(noThanks)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(assistantPage), ui.LeftClick(noThanks)),
	)(ctx); err != nil {
		s.Fatal("Failed to test oobe Arc: ", err)
	}

	if err := uiauto.Combine("Go through the gesture flow",
		uiauto.IfSuccessThen(ui.WaitUntilExists(getStarted), ui.LeftClick(getStarted)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(next), ui.LeftClick(next)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(next), ui.LeftClick(next)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(next), ui.LeftClick(next)),
		uiauto.IfSuccessThen(ui.WaitUntilExists(getStarted), ui.LeftClick(getStarted)),
	)(ctx); err != nil {
		s.Fatal("Failed to test oobe Arc tablet flow: ", err)
	}
}
