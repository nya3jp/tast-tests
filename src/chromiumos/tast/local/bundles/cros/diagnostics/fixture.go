// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"

	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "diagnosticsPrep",
		Desc: "Ensure relevant service is running before diagnostics ui test",
		Contacts: []string{
			"zhangwenyu@google.com",       // Fixture maintainer
			"ashleydp@google.com",         // Fixture maintainer
			"cros-peripherals@google.com", // team mailing list
		},
		Impl:            newDiagnosticsPrepFixture(),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// diagnosticsPrepFixture is a fixture to ensure relevant service is running before diagnostics ui test
type diagnosticsPrepFixture struct {
	cr *chrome.Chrome
}

func newDiagnosticsPrepFixture() testing.FixtureImpl {
	return &diagnosticsPrepFixture{}
}

func (f *diagnosticsPrepFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	if err := utils.EnsureCrosHealthdRunning(ctx); err != nil {
		s.Fatal("Failed to ensure cros healthd running: ", err)
	}

	f.cr = cr
	return cr
}

func (f *diagnosticsPrepFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *diagnosticsPrepFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *diagnosticsPrepFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *diagnosticsPrepFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
