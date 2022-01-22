// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func: Gesture,
		Desc: "Smoke test that clicks through OOBE",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func Gesture(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--force-tablet-mode=touch_view", "--vmodule=wizard_controller=1"),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

	syncScreenButton := nodewith.Name("Turn on sync").Role(role.Button)
	if err := uiauto.Combine("Click next on the sync screen",
		ui.WaitUntilExists(syncScreenButton),
		ui.LeftClick(syncScreenButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click sync screen next button: ", err)
	}

	assistantScreenButton := nodewith.Name("No thanks").Role(role.Button)
	if err := uiauto.Combine("Click next on the assistant screen",
		ui.WaitUntilExists(assistantScreenButton),
		ui.LeftClick(assistantScreenButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click assistant screen next button: ", err)
	}
	if err := uiauto.Combine("Click next on the assistant screen",
		ui.WaitUntilExists(assistantScreenButton),
		ui.LeftClick(assistantScreenButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click assistant screen next button: ", err)
	}
	if err := uiauto.Combine("Click next on the assistant screen",
		ui.WaitUntilExists(assistantScreenButton),
		ui.LeftClick(assistantScreenButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click assistant screen next button: ", err)
	}
}
