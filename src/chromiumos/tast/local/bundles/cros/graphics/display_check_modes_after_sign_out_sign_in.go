// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	keyDelay    = 0.3
	logoutDelay = 20
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisplayCheckModesAfterSignOutSignIn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "To Check the display mode is preserved after sign out and signin",
		Contacts:     []string{"markyacoub@google.com", "chromeos-gfx-display@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.LoginTimeout + time.Minute,
	})
}

func DisplayCheckModesAfterSignOutSignIn(ctx context.Context, s *testing.State) {

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const (
		username = "testuser@gmail.com"
		password = "testpass"
	)

	// Log in to Chrome
	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection")
	}

	// Log out of Chrome
	if err := logout(ctx, tconn); err != nil {
		s.Fatal("Failed to logout: ", err)
	}

}

func logout(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the keyboard")
	}
	defer kb.Close()

	return uiauto.Combine("press the keyboard to logout",
		kb.AccelAction("Shift+Ctrl+Q"),
		kb.AccelAction("Shift+Ctrl+Q"),
	)(ctx)
}
