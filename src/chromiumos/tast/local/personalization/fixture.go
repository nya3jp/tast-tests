// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// NewPersonalizationFixture creates a new implementation of Personalization fixture.
func NewPersonalizationFixture(gaia *GaiaVars, opts ...chrome.Option) testing.FixtureImpl {
	return &personalizationFixture{
		gaia: gaia,
		opts: opts,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "personalizationDefault",
		Desc: "Login with Personalization Hub enabled and start test API connection",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Impl:            NewPersonalizationFixture(nil, chrome.EnableFeatures("PersonalizationHub")),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "personalizationWithDarkLightMode",
		Desc: "Login with Personalization Hub and Dark Light mode enabled and start test API connection",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Impl:            NewPersonalizationFixture(nil, chrome.EnableFeatures("PersonalizationHub", "DarkLightMode")),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "personalizationWithGaiaLogin",
		Desc: "Login using Gaia account with Personalization Hub enabled and start test API connection",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Impl: NewPersonalizationFixture(&GaiaVars{
			UserVar: "ambient.username",
			PassVar: "ambient.password",
		}, chrome.EnableFeatures("PersonalizationHub")),
		SetUpTimeout:    chrome.GAIALoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"ambient.username",
			"ambient.password",
		},
	})
}

// GaiaVars holds the secret variables for username and password for a GAIA login.
type GaiaVars struct {
	UserVar string // the secret variable for the GAIA username
	PassVar string // the secret variable for the GAIA password
}

type personalizationFixture struct {
	cr    *chrome.Chrome
	gaia  *GaiaVars
	tconn *chrome.TestConn
	opts  []chrome.Option
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
}

func (f *personalizationFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if f.gaia != nil {
		// Login into the device, using GAIA login.
		username := s.RequiredVar(f.gaia.UserVar)
		password := s.RequiredVar(f.gaia.PassVar)
		f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{
			User: username,
			Pass: password,
		}))
	}

	clearUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	var err error

	f.cr, err = chrome.New(ctx, f.opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	f.tconn, err = f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(clearUpCtx, s.OutDir(), s.HasError, f.tconn)

	chrome.Lock()
	return FixtData{Chrome: f.cr, TestAPIConn: f.tconn}
}

func (f *personalizationFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
	f.tconn = nil
}

func (f *personalizationFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *personalizationFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *personalizationFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
