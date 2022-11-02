// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
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

	// Log in to Chrome
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection")
	}

	// Log out of Chrome
	if err := quicksettings.SignOut(ctx, tconn); err != nil {
		s.Fatal("Failed to logout: ", err)
	}
}
