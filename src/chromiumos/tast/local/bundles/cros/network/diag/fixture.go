// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diag

import (
	"context"
	"time"

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
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  1 * time.Second,
		PostTestTimeout: 1 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            &networkDiagnosticsFixture{},
		Parent:          "chromeLoggedIn",
	})
}

// networkDiagnosticsFixture implements testing.FixtureImpl.
type networkDiagnosticsFixture struct {
	api  *MojoAPI
	conn *chrome.Conn
}

func (f *networkDiagnosticsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr := s.ParentValue().(*chrome.Chrome)

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
	f.conn = conn

	api, err := NewMojoAPI(ctx, conn)
	if err != nil {
		s.Fatal("Unable to get network diagnostics mojo API: ", err)
	}

	f.api = api
	return f.api
}

func (f *networkDiagnosticsFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *networkDiagnosticsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *networkDiagnosticsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *networkDiagnosticsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.api.Release(ctx); err != nil {
		s.Log("Error releasing Network Diagnostics mojo API: ", err)
	}
	if err := f.conn.Close(); err != nil {
		s.Log("Error closing Chrome connection to app: ", err)
	}
}
