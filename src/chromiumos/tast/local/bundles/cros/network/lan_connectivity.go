// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	//"chromiumos/tast/local/apps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/connectivitydiagnosticsapp"
	"chromiumos/tast/testing"
)

// Example: patchpanel client for an example on how to set this up. It is
// similar to chrome.go.
func init() {
	testing.AddTest(&testing.Test{
		Func: LanConnectivity,
		Desc: "Run the LanConnectivity routine exposed by NetworkDiagnosticsRoutines Mojo API in Chrome",
		Contacts: []string{
			"khegde@google.com",
			"stevenjb@google.com",
			"tbegin@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"network_diagnostics_connection.js"},
	})
}

func LanConnectivity(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("ConnectivityDiagnosticsWebUi"),
	)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Launch the Connectivity Diagnostics app.
	app, err := connectivitydiagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
	defer app.Release(cleanupCtx)

	// TODO(khegde): How can I grab the connection object to the Web UI so I can run JS code
	// like I would by right-clicking->Inspect->Console on my app.

	// networkConn = [code - assume successful connection to Connectivity
	// Diagnostics Web UI has been made here...]
	
	// Can try creating a connection to chrome://connectivity-diagnostics - this doesn't work.
	/*networkConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://connectivity-diagnostics"))
	if err != nil {
		s.Fatal("Failed to set up Chrome conn to chrome://connectivity-diagnostics/")
	}
	defer networkConn.Close()*/

	// Javascript to run the routines.
	/*if err := networkConn.WaitForExpr(ctx, `chromeos.networkDiagnostics.mojom !== undefined`); err != nil {
		s.Fatal("Failed waiting for chromeos.networkDiagnostics.mojom to load: ", err)
	}
	js, err := ioutil.ReadFile(s.DataPath("network_diagnostics_connection.js"))
	if err != nil {
		s.Fatal("Failed to load JS for running routines: ", err)
	}

	// Set up an object to invoke network diagnostic routines.
	var networkDiagnosticsRoutines chrome.JSObject
	if err := networkConn.Call(ctx, &networkDiagnosticsRoutines, string(js)); err != nil {
		s.Fatal("Failed to set up the network diagnostic routines object: ", err)
	}

	// Run the lanConnectivity routine.
	if err := networkDiagnosticsRoutines.Call(ctx, nil, `async function() {await this.lanConnectivity()}`); err != nil {
		s.Fatal("Failed to run lanConnectivity routine: ", err)
	}*/
}
