// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture defines fixtures for inputs tests.
package fixture

import (
	"context"
	"net/http/httptest"
	"path/filepath"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/useractions"
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
		TearDownTimeout: chrome.ResetTimeout,
		ResetTimeout:    chrome.ResetTimeout,
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
		TearDownTimeout: chrome.ResetTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name: KioskVK,
		Desc: "Fixture should be used to test virtual keyboard typing in kiosk mode (ash chrome) with e14s-test page loaded",
		Contacts: []string{
			"jhtin@chromium.org",
			"alt-modalities-stability@google.com",
		},
		Impl: &inputsKioskFixture{
			extraOpts: []chrome.Option{chrome.VKEnabled()},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosKioskVK,
		Desc: "Fixture should be used to test virtual keyboard typing in kiosk mode (lacros chrome) with e14s-test page loaded",
		Contacts: []string{
			"jhtin@chromium.org",
			"alt-modalities-stability@google.com",
		},
		Impl: &inputsKioskFixture{
			extraOpts: []chrome.Option{chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore")},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
}

type inputsKioskFixture struct {
	cr         *chrome.Chrome
	testserver *httptest.Server
	kiosk      *kioskmode.Kiosk
	extraOpts  []chrome.Option
	tconn      *chrome.TestConn
	uc         *useractions.UserContext
}

// InputsKioskFixtData is the data returned by kiosk fixture SetUp and passed to tests.
type InputsKioskFixtData struct {
	chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	UserContext *useractions.UserContext
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
	k.testserver = testserver.LaunchServer(ctx)

	// Creating a kiosk mode configuration that will launch an app that points to the e14s-test page.
	webKioskAccountID := "arbitrary_id_web_kiosk_1@managedchrome.com"
	webKioskAccountType := policy.AccountTypeKioskWebApp
	webKioskTitle := "TastKioskModeSetByPolicyE14sPage"
	webKioskURL := k.testserver.URL + "/e14s-test"
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
	k.cr = cr
	k.kiosk = kiosk

	k.tconn, err = k.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), k.cr, k.tconn, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create new inputs user context")
	}
	k.uc = uc

	chrome.Lock()
	return InputsKioskFixtData{k.cr, k.tconn, k.uc}
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
	k.tconn = nil
}

func (k *inputsKioskFixture) Reset(ctx context.Context) error {
	if err := k.tconn.Eval(ctx, "chrome.tabs.reload()", nil); err != nil {
		return errors.Wrap(err, "failed to run chrome.tabs.reload()")
	}

	if err := k.tconn.WaitForExpr(ctx, "document.readyState == 'complete'"); err != nil {
		return errors.Wrap(err, "document did not load after reload")
	}
	return nil
}

func (k *inputsKioskFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// filepath.Base(s.OutDir()) returns the test name.
	// TODO(b/235164130) use s.TestName once it is available.
	k.uc.SetTestName(filepath.Base(s.OutDir()))
}

func (k *inputsKioskFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
