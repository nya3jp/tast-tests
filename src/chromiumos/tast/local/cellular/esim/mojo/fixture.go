// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mojo

import (
	"context"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithMojoTestEuicc",
		Desc: "Logs into a user session and creates a JS object for accessing mojo eSIM API calls for test eUICCS",
		Contacts: []string{
			"jstanko@google.com",
			"cros-connectivity@google.com",
		},
		Impl:            newESimMojoFixture(testEuicc()),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithMojoEuicc",
		Desc: "Logs into a user session and creates a JS object for accessing mojo eSIM API calls for eUICCS",
		Contacts: []string{
			"jstanko@google.com",
			"cros-connectivity@google.com",
		},
		Impl:            newESimMojoFixture(),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

type option func(*eSimMojoFixture)

func newESimMojoFixture(opts ...option) *eSimMojoFixture {
	f := &eSimMojoFixture{}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

func testEuicc() func(*eSimMojoFixture) {
	return func(e *eSimMojoFixture) {
		e.isTestEuicc = true
	}
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	Cr      *chrome.Chrome
	Manager *ESimManager
	Euicc   *Euicc
}

// eSimMojoFixture implements testing.FixtureImpl.
type eSimMojoFixture struct {
	manager     *ESimManager
	cr          *chrome.Chrome
	isTestEuicc bool
}

func (f *eSimMojoFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	return nil
}

func (*eSimMojoFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*eSimMojoFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *eSimMojoFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	hEuicc, slot, err := hermes.GetEUICC(ctx, f.isTestEuicc)
	if err != nil {
		s.Fatal("Failed to get eUICC via hermes: ", err)
	}

	if err := hEuicc.DBusObject.Call(ctx, hermesconst.EuiccMethodUseTestCerts, f.isTestEuicc).Err; err != nil {
		s.Fatal("Failed to set use test cert on eUICC: ", err)
	}

	var chromeOpts []chrome.Option
	if f.isTestEuicc {
		chromeOpts = append(chromeOpts, chrome.EnableFeatures("UseStorkSmdsServerAddress"))
		if slot == 1 {
			chromeOpts = append(chromeOpts, chrome.EnableFeatures("CellularUseSecondEuicc"))
		}
	}

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/internet")
	if err != nil {
		s.Fatal("Failed to open settings app: ", err)
	}

	var js chrome.JSObject

	if err := conn.Call(ctx, &js, ESimManagerJS); err != nil {
		s.Fatal("Failed to create eSIM mojo JS object: ", err)
	}

	f.cr = cr
	f.manager = &ESimManager{js: &js}

	euiccs, err := f.manager.AvailableEuicc(ctx)
	if err != nil {
		s.Fatal("Failed to get available eUICCs via Mojo: ", err)
	}

	if slot >= len(euiccs) {
		s.Fatal("Failed to determine correct eUICC, slot index out of range")
	}

	euicc := &euiccs[slot]

	euiccProperties, err := euicc.Properties(ctx)
	if err != nil {
		s.Fatal("Error getting eUICCs properties via Mojo: ", err)
	}
	s.Log("Using eUICC: ", euiccProperties.Eid)

	return &FixtData{Euicc: euicc, Manager: f.manager, Cr: f.cr}
}

func (f *eSimMojoFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.manager.js.Release(ctx); err != nil {
		s.Fatal("Failed to release eSIM Mojo JS object: ", err)
	}

	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
}
