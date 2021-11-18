// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
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
			"chromeos-commercial-remote-management@google.com",
		},
		Impl: &mgsFixtureState{
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
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
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
			"chromeos-commercial-remote-management@google.com",
		},
		Impl: &mgsFixtureState{
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
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
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
	// vdiApplicationToStart is the VDI application that is launched.
	vdiApplicationToStart apps.App
	// vdiConnector
	vdiConnector vdiApps.VdiInt
	// vdiServerKey is a key to retrieve VDI server url.
	vdiServerKey string
	// vdiUsernameKey is a key to retrieve user name to access VDI app.
	vdiUsernameKey string
	// vdiPasswordKey is a key to retrieve VDI user password.
	vdiPasswordKey string
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

	vdiAccountID := "mgs_vdi"
	accountType := policy.AccountTypePublicSession
	mgsAppPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &vdiAccountID,
		AccountType: &accountType,
	}

	accountsConfiguration := policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			mgsAppPolicy,
		},
	}

	installPolicy := policy.ExtensionInstallForcelist{Val: []string{
		v.vdiApplicationToStart.ID,
	}}
	pinPolicy := policy.PinnedLauncherApps{Val: []string{
		v.vdiApplicationToStart.ID,
	}}

	// TODO: Replace with mgs.New() after b/204177106 is done.
	err := func(ctx context.Context) error {
		testing.ContextLog(ctx, "mgs - starting Chrome to set mgs policies")
		cr, err := chrome.New(
			ctx,
			chrome.FakeLogin(chrome.Creds{User: Username, Pass: Password}), // Required as refreshing policies require test API.
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepEnrollment(),
		)
		if err != nil {
			return errors.Wrap(err, "failed to start Chrome")
		}

		// Set local accounts policy.
		policies := []policy.Policy{
			&accountsConfiguration,
		}

		// Use auto login.
		policies = append(policies, &policy.DeviceLocalAccountAutoLoginId{
			Val: vdiAccountID,
		})

		pb := fakedms.NewPolicyBlob()
		pb.AddPolicies(policies)
		// Handle public account policies.
		pb.AddPublicAccountPolicies(vdiAccountID, []policy.Policy{
			&installPolicy,
			&pinPolicy,
		})

		// Update policies.
		if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
			return errors.Wrap(err, "failed to serve and refresh policies")
		}

		// Close the previous Chrome instance.
		defer cr.Close(ctx)
		return nil
	}(ctx)
	if err != nil {
		s.Fatal(err, "failed preparing Chrome to start with given Kiosk configuration")
	}

	testing.ContextLog(ctx, "mgs - starting Chrome in mgs")
	// Restart Chrome. After that MGS auto starts.
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Fatal(err, "Chrome restart failed")
	}

	v.cr = cr

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "vdi_mgs_fixt_ui_tree")

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Wait for apps to be installed before launching")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, v.vdiApplicationToStart.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for app to install: ", err)
	}

	testing.ContextLog(ctx, "Starting VDI app")
	if err := apps.Launch(ctx, tconn, v.vdiApplicationToStart.ID); err != nil {
		s.Fatal("Failed to launch vdi app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, v.vdiApplicationToStart.ID, time.Minute); err != nil {
		s.Fatal("The VDI app did not appear in shelf after launch: ", err)
	}

	detector := uidetection.New(tconn,
		s.RequiredVar("uidetection.key_type"),
		s.RequiredVar("uidetection.key"),
		s.RequiredVar("uidetection.server"))
	v.vdiConnector.Init(s, detector)

	k, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer k.Close()

	if err := v.vdiConnector.Login(
		ctx,
		k,
		&vdiApps.VDILoginConfig{
			Server:   s.RequiredVar(v.vdiServerKey),
			Username: s.RequiredVar(v.vdiUsernameKey),
			Password: s.RequiredVar(v.vdiPasswordKey),
		}); err != nil {
		s.Fatal("Was not able to login to the vdi application: ", err)
	}
	if err := v.vdiConnector.EnsureMainScreenVisible(ctx); err != nil {
		s.Fatal("Main screen of the vdi application was not visible: ", err)
	}

	chrome.Lock()
	return &FixtureData{vdiConnector: v.vdiConnector, cr: cr, uidetector: detector}
}

func (v *mgsFixtureState) TearDown(ctx context.Context, s *testing.FixtState) {
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
	if err := v.vdiConnector.EnsureMainScreenVisible(ctx); err != nil {
		return errors.Wrap(err, "VDi main screen was not present")
	}

	return nil
}

func (v *mgsFixtureState) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (v *mgsFixtureState) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := v.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create TestAPI connection: ", err)
	}

	policies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain policies from Chrome: ", err)
	}

	b, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		s.Fatal("Failed to marshal policies: ", err)
	}

	// Dump all policies as seen by Chrome to the tests OutDir.
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), fixtures.PolicyFileDump), b, 0644); err != nil {
		s.Error("Failed to dump policies to file: ", err)
	}
}
