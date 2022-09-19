// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
		Impl: newDiagnosticsPrepFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 15*time.Second,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PreTestTimeout:  15 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "diagnosticsPrepForInputDiagnostics",
		Desc: "Ensure relevant service is running before diagnostics ui test",
		Contacts: []string{
			"zhangwenyu@google.com",       // Fixture maintainer
			"ashleydp@google.com",         // Fixture maintainer
			"dpad@google.com",             // Fixture maintainer
			"cros-peripherals@google.com", // team mailing list
		},
		Impl: newDiagnosticsPrepFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("EnableInputInDiagnosticsApp")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 15*time.Second,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PreTestTimeout:  15 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})
}

const appURL = "chrome://diagnostics/"

// FixtureData contains the data available for use in diagnostics tests
type FixtureData struct {
	Cr    *chrome.Chrome
	Tconn *chrome.TestConn
}

// diagnosticsPrepFixture is a fixture to ensure relevant service is running
// before diagnostics ui test.
type diagnosticsPrepFixture struct {
	cr    *chrome.Chrome
	api   *MojoAPI
	tconn *chrome.TestConn
	fopt  chrome.OptionsCallback
}

func newDiagnosticsPrepFixture(fopt chrome.OptionsCallback) testing.FixtureImpl {
	return &diagnosticsPrepFixture{fopt: fopt}
}

func (f *diagnosticsPrepFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	opts, err := f.fopt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain Chrome options: ", err)
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer func() {
		if !success {
			cr.Close(ctx)
		}
	}()

	if err := EnsureCrosHealthdRunning(ctx); err != nil {
		s.Fatal("Failed to ensure cros healthd running: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	success = true
	f.cr = cr
	f.tconn = tconn

	return &FixtureData{Cr: cr, Tconn: tconn}
}

func (f *diagnosticsPrepFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome: ", err)
	}

	f.cr = nil
	f.tconn = nil
}

func (f *diagnosticsPrepFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *diagnosticsPrepFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	success := false

	if _, err := diagnosticsapp.Launch(ctx, f.tconn); err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}

	conn, err := f.cr.NewConnForTarget(ctx, chrome.MatchTargetURL(appURL))
	if err != nil {
		s.Fatal("Failed to match the diagnostics chrome connection: ", err)
	}

	// Make sure mojo API is connected.
	var api *MojoAPI
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if api, err = SystemDataProviderMojoAPI(ctx, conn); err != nil {
			return errors.Wrap(err, "unable to get systemDataProvider mojo API")
		}

		if err := api.RunFetchSystemInfo(ctx); err != nil {
			return errors.Wrap(err, "failed to fetch system info")
		}

		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to connect to mojo API: ", err)
	}

	defer func() {
		if !success {
			api.Release(ctx)
		}
	}()

	success = true
	f.api = api
}

func (f *diagnosticsPrepFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, f.cr, "ui_dump")

	if err := f.api.Release(ctx); err != nil {
		s.Log("Error releasing systemDataProvider mojo API: ", err)
	}

	if err := diagnosticsapp.Close(ctx, f.tconn); err != nil {
		s.Log("Failed to close diagnostics app: ", err)
	}

	f.api = nil
}
