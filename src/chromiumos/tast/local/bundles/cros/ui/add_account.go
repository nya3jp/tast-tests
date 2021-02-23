// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AddAccount,
		Desc: "Follows the user flow to change the wallpaper",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInToCUJUserKeepState",
	})
}

func AddAccount(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("People").Role(role.Link))
	if err != nil {
		s.Fatal("Failed to lanuch setting page to add account: ", err)
	}

	ui := uiauto.New(tconn)
	accountsButton := nodewith.Name("Google Accounts").Role(role.Button)
	if err := uiauto.Run(ctx,
		settings.FocusAndWait(accountsButton),
		settings.LeftClick(accountsButton),
		ui.LeftClick(nodewith.Name("Add account").Role(role.Button)),
	); err != nil {
		s.Fatal("Failed to add new google account: ", err)
	}

	conn, err := cr.NewConnForTarget(ctx, func(t *target.Info) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	})

	// signConn, err := cr.NewConnForTarget(ctx, func(t *target.Info) bool {
	// 	return t.Type == "webview" && strings.HasPrefix(t.URL, "chrome://chrome-signin")
	// })

	testing.ContextLog(ctx, conn)

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	// insertGAIAField fills a field of the GAIA login form.
	insertGAIAField := func(ctx context.Context, gaiaConn *chrome.Conn, selector, value string) error {
		// Ensure that the input exists.
		if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
			"document.querySelector(%q)", selector)); err != nil {
			return errors.Wrapf(err, "failed to wait for %q element", selector)
		}
		// Ensure the input field is empty.
		// This confirms that we are not using the field before it is cleared.
		fieldReady := fmt.Sprintf(`
				(function() {
					const field = document.querySelector(%q);
					return field.value === "";
					})()`, selector)
		if err := gaiaConn.WaitForExpr(ctx, fieldReady); err != nil {
			return errors.Wrapf(err, "failed to wait for %q element to be empty", selector)
		}

		// Fill the field with value.
		if err := gaiaConn.Call(ctx, nil, `(selector, value) => {
			 	const field = document.querySelector(selector);
			 	field.value = value;
			}	`, selector, value); err != nil {
			return errors.Wrapf(err, "failed to use %q element", selector)
		}
		return nil
	}

	if err := insertGAIAField(ctx, conn, "#identifierId", "videodut0@cienetqa.education"); err != nil {
		s.Fatal("Failed to fill username field")
	}

	nextButton := nodewith.Name("Next").Role(role.Button)
	if err := uiauto.Run(ctx,
		settings.FocusAndWait(nextButton),
		settings.LeftClick(nextButton),
		ui.LeftClick(nodewith.Name("Add account").Role(role.Button)),
	); err != nil {
		s.Fatal("Failed to add account: ", err)
	}

	if err := insertGAIAField(ctx, conn, "input[name=password]", "CrosDutC0nn3ct"); err != nil {
		s.Fatal("Failed to fill in password field")
	}

}
