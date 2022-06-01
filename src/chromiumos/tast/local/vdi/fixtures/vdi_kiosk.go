// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/uidetection"
	vdiApps "chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/local/vdi/apps/citrix"
	"chromiumos/tast/local/vdi/apps/vmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.KioskCitrixLaunched,
		Desc: "Starts DUT fake enrolled in Kiosk mode with Citrix application installed, started and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &kioskFixtureState{
			vdiApplicationToStart: apps.Citrix,
			vdiConnector:          &citrix.Connector{},
			vdiServerKey:          "vdi.citrix_url",
			vdiUsernameKey:        "vdi.citrix_username",
			vdiPasswordKey:        "vdi.citrix_password",
		},
		Vars: []string{
			"vdi.citrix_url",
			"vdi.citrix_username",
			"vdi.citrix_password",
			"uidetection.key_type",
			"uidetection.key",
			"uidetection.server",
		},
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + vdiApps.VDILoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Data:            citrix.CitrixData,
		Parent:          fixture.FakeDMSEnrolled,
	})

	testing.AddFixture(&testing.Fixture{
		Name: fixture.KioskVmwareLaunched,
		Desc: "Starts DUT fake enrolled in Kiosk mode with Vmware application installed, started and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &kioskFixtureState{
			vdiApplicationToStart: apps.VMWare,
			vdiConnector:          &vmware.Connector{},
			vdiServerKey:          "vdi.vmware_url",
			vdiUsernameKey:        "vdi.vmware_username",
			vdiPasswordKey:        "vdi.vmware_password",
		},
		Vars: []string{
			"vdi.vmware_url",
			"vdi.vmware_username",
			"vdi.vmware_password",
			"uidetection.key_type",
			"uidetection.key",
			"uidetection.server",
		},
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + vdiApps.VDILoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Data:            vmware.VmwareData,
		Parent:          fixture.FakeDMSEnrolled,
	})
}

type kioskFixtureState struct {
	// cr is a connection to an already-started Chrome instance that loads
	// policies from FakeDMS.
	cr *chrome.Chrome
	// kiosk is a reference to a kiosk sessions providing a clean way to tear
	// down the session.
	kiosk *kioskmode.Kiosk
	// vdiApplicationToStart is the VDI application that is launched.
	vdiApplicationToStart apps.App
	// VdiApplication
	vdiConnector vdiApps.VDIInt
	// vdiServerKey is a key to retrieve VDI server url.
	vdiServerKey string
	// vdiUsernameKey is a key to retrieve user name to access VDI app.
	vdiUsernameKey string
	// vdiPasswordKey is a key to retrieve VDI user password.
	vdiPasswordKey string
	// accountsConfiguration holds the Kiosk accounts configuration that is
	// prepared in SetUp when creating Kiosk session and in TearDown when
	// cleaning policies preventing Kiosk from autostart - since
	// kioskmode.AutoLaunch() is used.
	accountsConfiguration policy.DeviceLocalAccounts
}

func (v *kioskFixtureState) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	vdiAccountID := "vdi_kiosk@managedchrome.com"
	accountType := policy.AccountTypeKioskApp
	kioskAppPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &vdiAccountID,
		AccountType: &accountType,
		KioskAppInfo: &policy.KioskAppInfo{
			AppId: &v.vdiApplicationToStart.ID,
		}}

	v.accountsConfiguration = policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			kioskAppPolicy,
		},
	}

	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.CustomLocalAccounts(&v.accountsConfiguration),
		kioskmode.AutoLaunch(vdiAccountID),
	)
	if err != nil {
		s.Error("Failed to start Chrome in kiosk mode: ", err)
	}

	ok = false
	defer func(ctx context.Context) {
		if !ok {
			if err := kiosk.Close(ctx); err != nil {
				s.Error("Failed to close kiosk: ", err)
			}
		}
	}(ctx)

	v.cr = cr
	v.kiosk = kiosk

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "vdi_kiosk_fixt_ui_tree")

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	detector := uidetection.New(tconn,
		s.RequiredVar("uidetection.key_type"),
		s.RequiredVar("uidetection.key"),
		s.RequiredVar("uidetection.server"))
	v.vdiConnector.Init(s, detector)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer kb.Close()

	if err := v.vdiConnector.Login(
		ctx,
		kb,
		&vdiApps.VDILoginConfig{
			Server:   s.RequiredVar(v.vdiServerKey),
			Username: s.RequiredVar(v.vdiUsernameKey),
			Password: s.RequiredVar(v.vdiPasswordKey),
		}); err != nil {
		s.Fatal("Was not able to login to the VDI application: ", err)
	}
	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		s.Fatal("Main screen of the VDI application was not visible: ", err)
	}

	chrome.Lock()
	ok = true
	return &FixtureData{vdiConnector: v.vdiConnector, cr: cr, uidetector: detector, inKioskMode: true}
}

func (v *kioskFixtureState) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if v.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := v.kiosk.Close(ctx); err != nil {
		s.Error("Failed to close kiosk: ", err)
	}

	v.cr = nil
}

func (v *kioskFixtureState) Reset(ctx context.Context) error {
	// Check the connection to Chrome.
	if err := v.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	// Check the main VDI screen is on.
	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		return errors.Wrap(err, "VDI main screen was not present")
	}

	return nil
}

func (v *kioskFixtureState) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (v *kioskFixtureState) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := v.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create TestAPI connection: ", err)
	}

	if err := dumpPolicies(ctx, tconn, fixtures.PolicyFileDump); err != nil {
		s.Fatal("Could not store policies: ", err)
	}
}
