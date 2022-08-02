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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/uidetection"
	vdiApps "chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/local/vdi/apps/citrix"
	"chromiumos/tast/local/vdi/apps/vmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.MgsCitrixLaunched,
		Desc: "Starts DUT fake enrolled in MGS mode with Citrix application installed, started and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &mgsFixtureState{
			vdiApplicationToStart: apps.Citrix,
			vdiConnector:          &citrix.Connector{},
			vdiServerKey:          "vdi.citrix_url",
			vdiUsernameKey:        "vdi.citrix_username",
			vdiPasswordKey:        "vdi.citrix_password",
			useTape:               true,
		},
		Vars: []string{
			tape.ServiceAccountVar,
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
		Name: fixture.MgsVmwareLaunched,
		Desc: "Starts DUT fake enrolled in MGS mode with VMware application installed, started and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &mgsFixtureState{
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

type mgsFixtureState struct {
	// cr is a connection to an already-started Chrome instance that loads
	// policies from FakeDMS.
	cr *chrome.Chrome
	// keyboard is a reference to keyboard to be release in the TearDown.
	keyboard *input.KeyboardEventWriter
	// vdiApplicationToStart is the VDI application that is launched.
	vdiApplicationToStart apps.App
	// vdiConnector
	vdiConnector vdiApps.VDIInt
	// vdiServerKey is a key to retrieve VDI server url.
	vdiServerKey string
	// vdiUsernameKey is a key to retrieve user name to access VDI app.
	vdiUsernameKey string
	// vdiPasswordKey is a key to retrieve VDI user password.
	vdiPasswordKey string
	// useTape is a flag to use Tape for leasing a vdi account.
	useTape bool
	// tapeAccountManager is used for cleaning up Tape.
	tapeAccountManager *tape.GenericAccountManager
}

// Credentials used for authenticating the test user.
const (
	Username = "tast-user@managedchrome.com"
	Password = "test0000"
)

func (v *mgsFixtureState) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	vdiAccountID := "mgs_vdi@managedchrome.com"

	installPolicy := policy.ExtensionInstallForcelist{Val: []string{
		v.vdiApplicationToStart.ID,
	}}
	pinPolicy := policy.PinnedLauncherApps{Val: []string{
		v.vdiApplicationToStart.ID,
	}}
	// Using this policy will present a suggestion for logging out to be showed
	// when we close all the windows in PostTest.
	supressLoggingOutDialog := policy.SuggestLogoutAfterClosingLastWindow{Val: false}

	mgs, cr, err := mgs.New(ctx,
		fdms,
		mgs.Accounts(vdiAccountID),
		mgs.AutoLaunch(vdiAccountID),
		mgs.AddPublicAccountPolicies(vdiAccountID, []policy.Policy{
			&installPolicy,
			&pinPolicy,
			&supressLoggingOutDialog,
		}),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome in mgs session: ", err)
	}

	ok = false
	defer func(ctx context.Context) {
		if !ok {
			if err := mgs.Close(ctx); err != nil {
				s.Error("Failed to close mgs session: ", err)
			}
		}
	}(ctx)

	v.cr = cr

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "vdi_mgs_fixt_ui_tree")

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("VDI mgs: wait for apps to be installed before launching")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, v.vdiApplicationToStart.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for app to install: ", err)
	}

	testing.ContextLog(ctx, "VDI mgs: starting VDI app")
	if err := apps.Launch(ctx, tconn, v.vdiApplicationToStart.ID); err != nil {
		s.Fatal("Failed to launch vdi app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, v.vdiApplicationToStart.ID, time.Minute); err != nil {
		s.Fatal("The VDI app did not appear in shelf after launch: ", err)
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
	return &FixtureData{vdiConnector: v.vdiConnector, cr: cr, uidetector: detector, inKioskMode: false}
}

func (v *mgsFixtureState) TearDown(ctx context.Context, s *testing.FixtState) {
	// Use a shortened context to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()

	if v.useTape {
		v.tapeAccountManager.CleanUp(cleanupCtx)
	}

	if err := v.vdiConnector.Logout(ctx); err != nil {
		s.Error("Couldn't logout from the VDI application: ", err)
	}

	v.keyboard.Close()
	chrome.Unlock()

	if v.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	// Clean the policies. This is important since
	// DeviceLocalAccountAutoLoginId policy is used to auto start MGS.
	// Otherwise with next Chrome start device polices (MGS account definition
	// and autologin policy) will be applied causing DUT to start MGS.
	if err := policyutil.ServeAndRefresh(ctx, fdms, v.cr, []policy.Policy{}); err != nil {
		s.Error("Failed to clean the policies: ", err)
	}

	if err := v.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	v.cr = nil
}

func (v *mgsFixtureState) Reset(ctx context.Context) error {
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

func (v *mgsFixtureState) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (v *mgsFixtureState) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := v.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all open windows: ", err)
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			s.Logf("Warning: Failed to close window (%+v): %v", w, err)
		}
	}

	testing.ContextLog(ctx, "VDI: Restarting VDI app")
	if err := apps.Launch(ctx, tconn, v.vdiApplicationToStart.ID); err != nil {
		s.Fatal("Failed to launch vdi app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, v.vdiApplicationToStart.ID, time.Minute); err != nil {
		s.Fatal("The VDI app did not appear in shelf after launch: ", err)
	}

	if err := v.vdiConnector.LoginAfterRestart(ctx); err != nil {
		s.Fatal("Couldn't log in after restart: ", err)
	}

	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		s.Fatal("VDI main screen was not present: ", err)
	}

	if err != nil {
		s.Fatal("Failed to create TestAPI connection: ", err)
	}

	if err := dumpPolicies(ctx, tconn, fixtures.PolicyFileDump); err != nil {
		s.Error("Could not dump policies: ", err)
	}
}
