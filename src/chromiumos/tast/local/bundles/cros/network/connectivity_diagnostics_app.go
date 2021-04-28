// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/conndiag"
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

// ConnectivityDiagnosticsApp ensures that the connectivity diagnostics
// application launches and displays the HTML.
func ConnectivityDiagnosticsApp(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	_, err = conndiag.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Error launching Connectivity Diagnostics App: ", err)
	}
}
