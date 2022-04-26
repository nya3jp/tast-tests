// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package terminal has tests for Terminal SSH System App.
package terminal

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SSH,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Terminal app can create an SSH outgoing client connection",
		Contacts: []string{
			"joelhockey@chromium.org",
			"chrome-hterm@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func SSH(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Start Chrome with TerminalSSH flag.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("TerminalSSH"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Get Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Open Terminal apps, first creating port forward for second to use.
	ta1, err := terminalapp.LaunchSSH(ctx, tconn, "chronos@localhost", "-L 8822:localhost:22", "test0000")
	if err != nil {
		s.Fatal("Failed to open Terminal app with port forwarding: ", err)
	}
	ta2, err := terminalapp.LaunchSSH(ctx, tconn, "chronos@localhost", "-p 8822", "test0000")
	if err != nil {
		s.Fatal("Failed to open Terminal app over port forwarding: ", err)
	}
	if err = uiauto.Combine("exit and cleanup",
		ta2.ExitSSH(),
		ta1.ExitSSH(),
	)(ctx); err != nil {
		s.Fatal("Failed to exit and cleanup: ", err)
	}
}
