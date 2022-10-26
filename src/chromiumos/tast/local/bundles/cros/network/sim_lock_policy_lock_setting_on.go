// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimLockPolicyLockSettingOn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test the notification flow that's triggered when the 'Lock SIM' setting is turned on before the policy is turned on",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable"},
		Fixture:      fixture.FakeDMSEnrolled,
		Timeout:      9 * time.Minute,
		Vars:         []string{"autotest_host_info_labels"},
	})
}

func SimLockPolicyLockSettingOn(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("SimLockPolicy"),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Perform clean up
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Gather Shill Device sim properties.
	labels, err := cellular.GetLabelsAsStringArray(ctx, s.Var, "autotest_host_info_labels")
	if err != nil {
		s.Fatal("Failed to read autotest_host_info_labels: ", err)
	}

	helper, err := cellular.NewHelperWithLabels(ctx, labels)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Enable and get service to set autoconnect based on test parameters.
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable modem")
	}

	iccid, err := helper.GetCurrentICCID(ctx)
	if err != nil {
		s.Fatal("Could not get current ICCID: ", err)
	}

	currentPin, currentPuk, err := helper.GetPINAndPUKForICCID(ctx, iccid)
	if err != nil {
		s.Fatal("Could not get Pin and Puk: ", err)
	}
	if currentPin == "" {
		// Do graceful exit, not to run tests on unknown pin duts.
		s.Fatal("Unable to find PIN code for ICCID: ", iccid)
	}
	if currentPuk == "" {
		// Do graceful exit, not to run tests on unknown puk duts.
		s.Fatal("Unable to find PUK code for ICCID: ", iccid)
	}

	// Check if pin enabled and locked/set.
	if helper.IsSimLockEnabled(ctx) || helper.IsSimPinLocked(ctx) {
		// Disable pin.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			s.Fatal("Failed to disable lock: ", err)
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	s.Log("Attempting to enable SIM lock with correct pin")
	if err = helper.Device.RequirePin(ctx, currentPin, true); err != nil {
		s.Fatal("Failed to enable pin, mostly default pin needs to set on dut: ", err)
	}

	defer func(ctx context.Context) {
		// Check if pin enabled and locked/set.
		if helper.IsSimLockEnabled(ctx) || helper.IsSimPinLocked(ctx) {
			// Disable pin.
			if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
				s.Fatal("Failed to turn off lock: ", err)
			}
			s.Fatal("PIN lock was not disabled through the UI")
		}
	}(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)

	globalConfig := &policy.ONCGlobalNetworkConfiguration{
		AllowCellularSimLock: false,
	}

	deviceNetworkPolicy := &policy.DeviceOpenNetworkConfiguration{
		Val: &policy.ONC{
			GlobalNetworkConfiguration: globalConfig,
			NetworkConfigurations:      []*policy.ONCNetworkConfiguration{},
		},
	}

	// Apply Global Network Configuration.
	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{deviceNetworkPolicy}); err != nil {
		s.Fatal("Failed to ServeAndRefresh ONC policy: ", err)
	}

	const notificationTitle = "Turn off \"Lock SIM\" setting"
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitTitle(notificationTitle)); err != nil {
		s.Fatalf("Failed waiting for %v: %v", notificationTitle, err)
	}

	ui := uiauto.New(tconn)
	popup := nodewith.Role(role.Window).HasClass("ash/message_center/MessagePopup")
	if err := ui.LeftClick(popup)(ctx); err != nil {
		s.Fatal("Failed to click on notification: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		s.Fatal("Failed to show settings: ", err)
	}

	if err := ui.WaitUntilExists(ossettings.EnterButton)(ctx); err != nil {
		s.Fatal("Waiting for EnterButton: ", err)
	}

	if err := ui.WaitUntilExists(ossettings.VisibilityButton)(ctx); err != nil {
		s.Fatal("Waiting for VisibilityButton: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer keyboard.Close()

	if err := keyboard.Type(ctx, currentPin); err != nil {
		s.Fatal("Could not type PIN: ", err)
	}

	if err := ui.LeftClick(ossettings.EnterButton)(ctx); err != nil {
		s.Fatal("Failed to click Enter button: ", err)
	}

	// Await for Visibility button to close.
	if err := ui.WaitUntilGone(ossettings.EnterButton)(ctx); err != nil {
		s.Fatal("Failed wait until Enter button is gone: ", err)
	}
}
