// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture defines fixtures for inputs tests.
package fixture

import (
	"context"
	"net/http/httptest"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

// List of fixture names for inputs using kiosk mode.
const (
	// KioskNonVK is the fixture for physical keyboard in Kiosk mode for ash.
	KioskNonVK = "kioskNonVK"
	// KioskNonVK is the fixture for physical keyboard in Kiosk mode for lacros.
	LacrosKioskNonVK = "lacrosKioskNonVK"
	// KioskNonVK is the fixture for virtual keyboard in Kiosk mode for ash.
	KioskVK = "kioskVK"
	// KioskNonVK is the fixture for virtual keyboard in Kiosk mode for lacros.
	LacrosKioskVK = "lacrosKioskVK"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: KioskNonVK,
		Desc: "Fixture should be used to test physical keyboard typing in kiosk mode (ash chrome) with e14s-test page loaded",
		Contacts: []string{
			"jhtin@chromium.org",
			"alt-modalities-stability@google.com",
		},
		Impl:            &inputsKioskFixture{},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosKioskNonVK,
		Desc: "Fixture should be used to test physical keyboard typing in kiosk mode (lacros chrome) with e14s-test page loaded",
		Contacts: []string{
			"jhtin@chromium.org",
			"alt-modalities-stability@google.com",
		},
		Impl: &inputsKioskFixture{
			extraOpts: []chrome.Option{chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore")},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name: KioskVK,
		Desc: "Fixture should be used to test physical keyboard typing in kiosk mode (lacros chrome) with e14s-test page loaded",
		Contacts: []string{
			"jhtin@chromium.org",
			"alt-modalities-stability@google.com",
		},
		Impl: &inputsKioskFixture{
			extraOpts: []chrome.Option{chrome.VKEnabled()},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosKioskVK,
		Desc: "Fixture should be used to test physical keyboard typing in kiosk mode (lacros chrome) with e14s-test page loaded",
		Contacts: []string{
			"jhtin@chromium.org",
			"alt-modalities-stability@google.com",
		},
		Impl: &inputsKioskFixture{
			extraOpts: []chrome.Option{chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore")},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
}

type inputsKioskFixture struct {
	cr         *chrome.Chrome
	testserver *httptest.Server
	kiosk      *kioskmode.Kiosk
	extraOpts  []chrome.Option
}

// InputsKioskFixtData is the data returned by kiosk fixture SetUp and passed to tests.
type InputsKioskFixtData struct {
	chrome *chrome.Chrome
}

// Chrome implements the HasChrome interface.
func (f InputsKioskFixtData) Chrome() *chrome.Chrome {
	if f.chrome == nil {
		panic("Chrome is called with nil chrome instance")
	}
	return f.chrome
}

func (k *inputsKioskFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a fakeDMSEnrolled fixture")
	}
	// Launches the server for e14s-test page.
	server := testserver.LaunchServer(ctx)

	// Creating a kiosk mode configuration that will launch an app that points to the e14s-test page.
	webKioskAccountID := "arbitrary_id_web_kiosk_1@managedchrome.com"
	webKioskAccountType := policy.AccountTypeKioskWebApp
	webKioskTitle := "TastKioskModeSetByPolicyE14sPage"
	webKioskURL := server.URL + "/e14s-test"
	webKioskPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &webKioskAccountID,
		AccountType: &webKioskAccountType,
		WebKioskAppInfo: &policy.WebKioskAppInfo{
			Url:     &webKioskURL,
			Title:   &webKioskTitle,
			IconUrl: &webKioskURL,
		}}

	localAccountsConfiguration := &policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			webKioskPolicy,
		},
	}

	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.AutoLaunch(webKioskAccountID),
		kioskmode.CustomLocalAccounts(localAccountsConfiguration),
		kioskmode.ExtraChromeOptions(k.extraOpts...),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}

	chrome.Lock()
	k.kiosk = kiosk
	k.cr = cr
	k.testserver = server
	return InputsKioskFixtData{k.cr}
}

func (k *inputsKioskFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if k.cr == nil {
		s.Log("Chrome not yet started")
	}

	if err := k.kiosk.Close(ctx); err != nil {
		s.Log("There was an error while closing Kiosk: ", err)
	}

	k.testserver.Close()
	k.cr = nil
}

func (k *inputsKioskFixture) Reset(ctx context.Context) error {
	// Restarts the kiosk app to reset the state.
	cr, err := k.kiosk.RestartChromeWithOptions(
		ctx,
		k.extraOpts...)
	if err != nil {
		return err
	}
	k.cr = cr
	return nil
}

func (k *inputsKioskFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (k *inputsKioskFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
