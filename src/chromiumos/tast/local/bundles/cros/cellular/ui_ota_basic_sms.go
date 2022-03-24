// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/modemmanager"

	//"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// This test is only run on the cellular_single_active group. All boards in that group
// provide the Modem.SimSlots property and have at least one provisioned SIM slot.

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIOtaBasicSms,
		Desc:         "Verifies that Shill can connect to a service in a different slot",
		Contacts:     []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		SoftwareDeps: []string{"chrome"},
		//Fixture:      "chromeLoggedIn",
		Vars: []string{
			"ui.oac_username",
			"ui.oac_password",
		},
	})
}

// UIOtaBasicSms validates MT SMS, uses google voice to send SMS
func UIOtaBasicSms(ctx context.Context, s *testing.State) {

	// Check cellular connection and get mobile number on dut
	// Create and send SMS on google voice ui from chrome web interface
	// Check UI notifications for SMS, verify SMS content
	// Clear received SMS

	messageToSend := "Hello " + time.Now().Format("01-02-2022 16:10:10")
	// Check cellular connection through shill or modemmanager(registered)
	s.Log("Check cellular state")

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}
	// Ensure that a Cellular Service was created.
	if _, err := helper.FindService(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service: ", err)
	}

	// OwnNumber
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create modem: ", err)
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on modem: ", err)
	}

	value, err := props.Get(mmconst.ModemPropertyOwnNumbers)
	if err != nil {
		s.Fatal("Missing ownnumbers property: ", err)
	}
	phoneNumbers, ok := value.([]string)
	if !ok {
		s.Fatal("Empty ownnumbers property: ", err)
	}

	s.Logf("Phone number: %s to send message: %s", phoneNumbers[0], messageToSend)

	// Create cleanup context to ensure UI tree dumps correctly.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	uiHelper, err := cellular.NewUIHelper(ctx, s)
	if err != nil {
		s.Fatal("Failed to create cellular.NewUiHelper: ", err)
	}

	s.Log("open google voice web page")

	// Create gv MO sms using google voice configured with owned test accounts.
	_, gConn, err := uiHelper.GoogleVoiceLogin(ctx)
	if err != nil {
		s.Fatal("Failed to open Google voice website: ", err)
	}
	defer gConn.Close()
	defer gConn.CloseTarget(cleanupCtx)

	if gConn == nil {
		s.Fatal("Could not create new chrome tab")
	}

	// Keyboard to input key inputs.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	s.Log("After finding messages tab")

	_, err = uiHelper.SendMessage(ctx, phoneNumbers[0], messageToSend)
	if err != nil {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, uiHelper.Tconn)
		s.Fatal("Failed to send message: ", err)
	}
	s.Log("Check for SMS message")
	_, err = uiHelper.ValidateMessage(ctx, messageToSend)
	if err != nil {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, uiHelper.Tconn)
		s.Fatal("Failed validation of message: ", err)
	}

	alertDialog := nodewith.Role(role.AlertDialog).ClassName("MessagePopupView").Onscreen()
	// Click on alert dialog to close
	if err := uiauto.Combine("Click on alert dialog",
		uiHelper.UI.WaitUntilExists(alertDialog),
		uiHelper.UIHandler.Click(alertDialog),
		kb.AccelAction("Ctrl+X"),
		uiHelper.UI.WaitUntilGone(alertDialog),
	)(ctx); err != nil {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, uiHelper.Tconn)
		s.Fatal("Failed to click on notification  dialog: ", err)
	}

}
