// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diag

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/conndiag"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "networkDiagnostics",
		Desc: "A network diagnostics mojo API is ready and available to use",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  1 * time.Second,
		PostTestTimeout: 1 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            newNetworkDiagnosticsFixture(),
	})
}

// networkDiagnosticsFixture implements testing.FixtureImpl.
type networkDiagnosticsFixture struct {
	api *MojoAPI
	cr  *chrome.Chrome
}

func newNetworkDiagnosticsFixture() testing.FixtureImpl {
	return &networkDiagnosticsFixture{}
}

// TODO(crbug/1127165): convert this to a data file when supported by fixtures.
const netDiagJs = `
/**
 * @fileoverview A wrapper file around the network diagnostics API.
 */
function() {
  return {
    /**
     * Network Diagnostics mojo remote.
     * @private {
     *     ?chromeos.networkDiagnostics.mojom.NetworkDiagnosticsRoutinesRemote}
     */
    networkDiagnostics_: null,

    getNetworkDiagnostics() {
      if (!this.networkDiagnostics_) {
        this.networkDiagnostics_ = chromeos.networkDiagnostics.mojom
                                       .NetworkDiagnosticsRoutines.getRemote()
      }
      return this.networkDiagnostics_;
    },

    async lanConnectivity() {
      return await this.getNetworkDiagnostics().lanConnectivity();
    },
  }
}
`

func (f *networkDiagnosticsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ConnectivityDiagnosticsWebUi"))
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	app, err := conndiag.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch connectivity diagnostics app: ", err)
	}

	conn, err := app.ChromeConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network diagnostics mojo: ", err)
	}

	var mojoRemote chrome.JSObject
	if err := conn.Call(ctx, &mojoRemote, netDiagJs); err != nil {
		s.Fatal("Failed to set up the network diagnostics mojo API: ", err)
	}

	f.cr = cr
	f.api = &MojoAPI{conn, &mojoRemote}

	return f.api
}

func (f *networkDiagnosticsFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	return nil
}

func (f *networkDiagnosticsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *networkDiagnosticsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *networkDiagnosticsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	errs := f.api.Release(ctx)
	for _, e := range errs {
		if e != nil {
			s.Log("Error releasing Network Diagnostics mojo API: ", e)
		}
	}

	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
}
