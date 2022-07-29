// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "diagnosticsPrep",
		Desc: "Ensure relevant server is running before diagnostics ui test",
		Contacts: []string{
			"zhangwenyu@google.com", // Fixture maintainer
		},
		Impl:            newDiagnosticsPrepFixture(),
		SetUpTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
	})
}

// diagnosticsPrepFixture is a fixture to ensure relevant server is running before diagnostics ui test
type diagnosticsPrepFixture struct {
	cr *Chrome
}

func newDiagnosticsPrepFixture() testing.FixtureImpl {
	return &diagnosticsPrepFixture{}
}

func (f *diagnosticsPrepFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return nil
}

func (f *diagnosticsPrepFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}

func (f *diagnosticsPrepFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *diagnosticsPrepFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("DiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	// defer cr.Close(ctx) // Close our own chrome instance

	if err := EnsureCrosHealthdRunning(ctx); err != nil {
		s.Fatal("Failed to ensure cros healthd running: ", err)
	}

	f.cr = cr
}

func (f *diagnosticsPrepFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}
