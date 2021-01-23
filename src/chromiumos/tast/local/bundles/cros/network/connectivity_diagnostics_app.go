// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConnectivityDiagnosticsApp,
		Desc: "Tests launching the connectivity diagnostics UI",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

var appRootParams = ui.FindParams{
	Name: "Connectivity Diagnostics",
	Role: ui.RoleTypeWindow,
}

var appTitleParams = ui.FindParams{
	Name: "Connectivity Diagnostics",
	Role: ui.RoleTypeInlineTextBox,
}

var pollOpts = testing.PollOptions{
	Interval: 100 * time.Millisecond,
	Timeout:  5 * time.Second,
}

// ConnectivityDiagnosticsApp ensures that the connectivity diagnostics
// application launches and displays the HTML.
func ConnectivityDiagnosticsApp(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Create a Chrome instance with the Connectivity Diagnostics WebUI app
	// enabled.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ConnectivityDiagnosticsWebUi"))
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := apps.Launch(ctx, tconn, apps.ConnectivityDiagnostics.ID); err != nil {
		s.Fatal("Failed to launch connectivity diagnostics app: ", err)
	}

	// Get the Connectivity Diagnostics app root node.
	appRoot, err := ui.FindWithTimeout(ctx, tconn, appRootParams, time.Minute)
	if err != nil {
		s.Fatal("Failed to find app root: ", err)
	}

	// Poll the root node for the title node.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err = appRoot.DescendantWithTimeout(ctx, appTitleParams, time.Minute)
		if err != nil {
			return errors.Wrap(err, "failed to find app title")
		}
		return nil
	}, &pollOpts); err != nil {
		s.Fatal("Failed to wait for app title: ", err)
	}
}
