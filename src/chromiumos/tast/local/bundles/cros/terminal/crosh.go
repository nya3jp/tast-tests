// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package terminal has tests for Terminal SSH System App.
package terminal

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Crosh,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify crosh System Web App",
		Contacts: []string{
			"joelhockey@chromium.org",
			"chrome-hterm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
	})
}

func Crosh(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Start Chrome with CroshSWA flag.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("CroshSWA"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Get Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Remove public IME if it is installed (seems to be only amd64-generic
	// boards) since it sends bad text and key events.
	const publicImeExt = "fgoepimhcoialccpbmpnnblemnepkkao"
	if err := tconn.Call(ctx, nil,
		"tast.promisify(chrome.autotestPrivate.removeComponentExtension)", publicImeExt); err != nil {
		s.Fatal("Failed to remove public IME extension: ", err)
	}

	// Launch Crosh.
	if err := apps.Launch(ctx, tconn, apps.Crosh.ID); err != nil {
		s.Fatal("Failed to launch: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")
	// Run shell, verify prompt, exit.
	ui := uiauto.New(tconn)
	err = uiauto.Combine("run crosh shell",
		ui.LeftClick(nodewith.Name("crosh").Role(role.Window).ClassName("BrowserFrame")),
		ui.WaitUntilExists(nodewith.Name("crosh>").Role(role.StaticText)),
		kb.TypeAction("shell"),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(nodewith.Name("chronos@localhost").Role(role.StaticText)),
		kb.TypeAction("exit"),
		kb.AccelAction("Enter"),
		kb.TypeAction("exit"),
		kb.AccelAction("Enter"),
		ui.WaitUntilGone(nodewith.Name("crosh>").Role(role.StaticText)),
	)(ctx)
	if err != nil {
		s.Fatal("Failed: ", err)
	}
}
