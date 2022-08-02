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
	"chromiumos/tast/common/tape"
	"chromiumos/tast/ctxutil"
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
			useTape:               true,
		},
		Vars: []string{
			tape.ServiceAccountVar,
			"vdi.citrix_url",
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
			useTape:               false,
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
	// keyboard is a reference to keyboard to be release in the TearDown.
	keyboard *input.KeyboardEventWriter
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
	// useTape is a flag to use Tape for leasing a vdi account.
	useTape bool
	// tapeAccountManager is used for cleaning up Tape.
	tapeAccountManager *tape.GenericAccountManager
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
		s.Fatal("Failed to start Chrome in kiosk mode: ", err)
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

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	// Keep it fo closing.
	v.keyboard = kb

	detector := uidetection.New(tconn,
		s.RequiredVar("uidetection.key_type"),
		s.RequiredVar("uidetection.key"),
		s.RequiredVar("uidetection.server"))
	v.vdiConnector.Init(s, tconn, detector, kb)

	var vdiUsername, vdiPassword string
	vdiServer := s.RequiredVar(v.vdiServerKey)

	if v.useTape {
		// Create an account manager and lease a vdi test account for the specified timeout as there are several sets in the scope of this fixture.
		accHelper, acc, err := tape.NewGenericAccountManager(ctx, []byte(s.RequiredVar(tape.ServiceAccountVar)), tape.WithTimeout(30*60), tape.WithPoolID(tape.Citrix))
		if err != nil {
			s.Fatal("Failed to create an account manager and lease a Citrix account: ", err)
		}
		v.tapeAccountManager = accHelper
		vdiUsername = CitrixUsernamePrefix + acc.Username
		vdiPassword = acc.Password
	} else {
		vdiUsername = s.RequiredVar(v.vdiUsernameKey)
		vdiPassword = s.RequiredVar(v.vdiPasswordKey)
	}

	if err := v.vdiConnector.Login(
		ctx,
		&vdiApps.VDILoginConfig{
			Server:   vdiServer,
			Username: vdiUsername,
			Password: vdiPassword,
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
	// Use a shortened context to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()

	if v.useTape {
		v.tapeAccountManager.CleanUp(cleanupCtx)
	}

	v.keyboard.Close()
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
