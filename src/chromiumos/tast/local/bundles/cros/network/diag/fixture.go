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
		SetUpTimeout:    chrome.LoginTimeout + (30 * time.Second),
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 10 * time.Second,
		Impl:            &networkDiagnosticsFixture{},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "networkDiagnosticsShillReset",
		Desc: "A network diagnostics mojo API is ready and available to use. This fixture also sets shill in a default state and resets any modifications",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SetUpTimeout:    chrome.LoginTimeout + (1 * time.Minute),
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 10 * time.Second,
		Impl:            &networkDiagnosticsFixture{},
		Parent:          "shillReset",
	})
}

// networkDiagnosticsFixture implements testing.FixtureImpl.
type networkDiagnosticsFixture struct {
	api  *MojoAPI
	conn *chrome.Conn
	cr   *chrome.Chrome
}

func (f *networkDiagnosticsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	// This fixture needs to create and manage its own Chrome instance to
	// ensure that the Connectivity Diagnostics app with the network diagnostics
	// mojo API is preserved between tests.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer func() {
		if !success {
			cr.Close(ctx)
		}
	}()

	app, err := conndiag.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch connectivity diagnostics app: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, app.Tconn)

	conn, err := app.ChromeConn(ctx)
	if err != nil {
		s.Fatal("Failed to get network diagnostics mojo: ", err)
	}
	defer func() {
		if !success {
			conn.Close()
		}
	}()

	api, err := NewMojoAPI(ctx, conn)
	if err != nil {
		s.Fatal("Unable to get network diagnostics mojo API: ", err)
	}
	defer func() {
		if !success {
			api.Release(ctx)
		}
	}()

	success = true
	f.cr = cr
	f.conn = conn
	f.api = api
	return f.api
}

func (f *networkDiagnosticsFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	return nil
}

func (f *networkDiagnosticsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *networkDiagnosticsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *networkDiagnosticsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.api.Release(ctx); err != nil {
		s.Log("Error releasing Network Diagnostics mojo API: ", err)
	}
	if err := f.conn.Close(); err != nil {
		s.Log("Error closing Chrome connection to app: ", err)
	}
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
}
