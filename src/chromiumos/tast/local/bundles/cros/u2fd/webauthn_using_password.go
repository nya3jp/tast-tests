// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebauthnUsingPassword,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that WebAuthn using password succeeds",
		Contacts: []string{
			"hcyang@google.com",
			"cros-hwsec@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "tpm1",
			ExtraSoftwareDeps: []string{"tpm1"},
		}, {
			Name:              "gsc",
			ExtraSoftwareDeps: []string{"gsc"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func WebauthnUsingPassword(ctx context.Context, s *testing.State) {
	if err := upstart.CheckJob(ctx, "u2fd"); err != nil {
		s.Fatal("u2fd isn't started: ", err)
	}

	// Try to get the system into a consistent state, since it seems like having
	// an already-mounted user dir can cause problems: https://crbug.com/963084
	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	const (
		username = fixtures.Username
		password = fixtures.Password
	)

	opts := []chrome.Option{
		chrome.FakeLogin(chrome.Creds{User: username, Pass: password}),
		// Enable device event log in Chrome logs for validation.
		chrome.ExtraArgs("--vmodule=device_event_log*=1")}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	authCallback := func(ctx context.Context, ui *uiauto.Context) error {
		// Check if the UI is correct.
		if err := ui.Exists(nodewith.ClassName("LoginPasswordView"))(ctx); err != nil {
			return errors.Wrap(err, "failed to find the password input field")
		}
		// Type password into ChromeOS WebAuthn dialog.
		if err := keyboard.Type(ctx, password+"\n"); err != nil {
			return errors.Wrap(err, "failed to type password into ChromeOS auth dialog")
		}
		return nil
	}

	if err := util.WebAuthnInSecurityKeysInfo(ctx, cr, authCallback); err != nil {
		s.Fatal("Failed to perform WebAuthn: ", err)
	}
}
