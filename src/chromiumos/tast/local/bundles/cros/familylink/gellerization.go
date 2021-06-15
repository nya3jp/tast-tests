// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gellerization,
		Desc:         "Checks if in-session gellerization flow is working",
		Contacts:     []string{"lienhoang@chromium.org", "cros-families-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 10*time.Minute,
		Vars: []string{
			"familylink.Gellerization.parentUser",
			"familylink.Gellerization.parentPassword",
			"familylink.Gellerization.childUser",
			"familylink.Gellerization.childPassword",
		},
	})
}

func Gellerization(ctx context.Context, s *testing.State) {
	childUser := s.RequiredVar("familylink.Gellerization.childUser")
	childPassword := s.RequiredVar("familylink.Gellerization.childPassword")

	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: childUser, Pass: childPassword}))

	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Launching the settings app")
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the settings app: ", err)
	}

	ui := uiauto.New(tconn)

	s.Log("Launching the gellerization flow")
	setUpButton := nodewith.Role(role.Button).Name("Parental controls")
	getStartedButton := nodewith.Name("Get started").Role(role.Button)
	if err := uiauto.Combine("open gellerization flow",
		ui.WaitUntilExists(setUpButton),
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(setUpButton, ui.Exists(getStartedButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to open gellerization flow: ", err)
	}

}
