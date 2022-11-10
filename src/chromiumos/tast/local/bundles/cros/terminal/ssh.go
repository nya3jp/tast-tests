// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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

	cr, err := chrome.New(ctx, chrome.EnableFeatures("TerminalAlternativeEmulator"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Get Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	// Open Terminal apps, first creating port forward for second to use.
	ta1, err := terminalapp.LaunchSSH(ctx, tconn, "-L 8822:localhost:22")
	if err != nil {
		s.Fatal("Failed to open ssh1: ", err)
	}
	if err := uiauto.Combine("change PS1 before opening 2nd terminal",
		ta1.RunSSHCommand("PS1='ssh1$ '"),
		ta1.RunSSHCommand("clear"),
	)(ctx); err != nil {
		s.Fatal("Failed to run command in ssh1: ", err)
	}
	ta2, err := terminalapp.LaunchSSH(ctx, tconn, "-p 8822")
	if err != nil {
		s.Fatal("Failed to open ssh2: ", err)
	}
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("pwd command",
		ta2.RunSSHCommand("pwd"),
		ui.WaitUntilExists(nodewith.Name("/home/chronos/user").Role(role.StaticText).First()),
		ta2.ExitSSH(),
	)(ctx); err != nil {
		s.Fatal("Failed to run command in ssh2: ", err)
	}
	if err := ta1.ExitSSH()(ctx); err != nil {
		s.Fatal("Failed to exit ssh1: ", err)
	}
}
