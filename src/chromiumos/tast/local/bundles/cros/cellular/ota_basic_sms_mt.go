// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// This test is only run on the cellular_single_active group. All boards in that group
// provide the Modem.SimSlots property and have at least one provisioned SIM slot.

func init() {
	testing.AddTest(&testing.Test{
		Func:     OtaBasicSmsMt,
		Desc:     "Verifies that Shill can connect to a service in a different slot",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		SoftwareDeps: []string{"chrome"},
		//Fixture:      "chromeLoggedIn",
	})
}

func OtaBasicSmsMt(ctx context.Context, s *testing.State) {

	// Check cellular connection for registered - Done
    // Create and send sms on google voice ui from chrome web interface
	// Check UI notifications for SMS, verify sms content - Done
	// Clear received sms - Done

	const (
		uiTimeout           = 60 * time.Second
		messageTitle        = "Chrome OS"
		messageReceivedText = "Hello" //"6692413363 Simple SMS"
		messageClear        = "Clear all" //as this appears for more than one message just * to close window
		username            = "testuser@gmail.com"
		password            = "good"
		voiceusername       = "movoicesms@gmail.com" //GVC No: 6692413363  connected no 6504174805 verizon
		voicepassword       = "User@2022"
		lockTimeout         = 30 * time.Second
		goodAuthTimeout     = 30 * time.Second
	)
	// Check cellular connection through shill or modemmanager(registered)
	s.Logf("Check cellular state")

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}
	// Ensure that a Cellular Service was created.
	if _, err := helper.FindService(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service: ", err)
	}

	// Create cleanup context to ensure UI tree dumps correctly.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()


	// Create gv MO sms - Study UI control based web logging and triggering
	const (
		launchVoice        = "https://voice.google.com/about" //or voice.google.com/u/0/messages browse in other tab
		gvoiceURL          = "https://voice.google.com/signup"
		emailText          = "Email or phone"
		nextText           = "Next"
		passwordText       = "Enter your password"
		logInText          = "Log In"
		noneOfTheAboveText = "NONE OF THE ABOVE"
	)

	//cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: voiceusername, Pass: voicepassword}),
		chrome.ProdPolicy(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	s.Log("xyz: about to test api conn")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// uiHandler will be assigned with different instances for clamshell and tablet mode.
	var uiHandler cuj.UIActionHandler
    if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("failed to create clamshell action handler", err)
	}

	defer uiHandler.Close()
	//uiHandler.LaunchChrome(ctx)
    s.Log("open google voice web page")
    // Open google voice tab
	openGvoiceWeb := func(ctx context.Context) (*chrome.Conn, error) {
		conn, err := uiHandler.NewChromeTab(ctx, cr, gvoiceURL, false)
		if err != nil {
			return conn, errors.Wrap(err, "failed to open voice web page")
		}
		if err := webutil.WaitForQuiescence(ctx, conn, 2*time.Minute); err != nil {
			return conn, errors.Wrap(err, "failed to wait for voice page to finish loading")
		}
		// Google Voice sometimes pops up a prompt to notice about notifications,
		// asking to allow/dismiss notifications.
		s.Log("Click ok button if exists")
		//allowPrompt := nodewith.Role(role.Button).Onscreen()
		okButton := nodewith.Name("OK").Role(role.Button).First()
		//okButton := nodewith.Role(role.Button).Onscreen().First()
		if err := uiauto.Combine("Click on ok button",
			ui.WaitUntilExists(okButton),
			ui.FocusAndWait(okButton),
			ui.LeftClick(okButton),
		)(ctx); err != nil {
			s.Fatal("Failed to click on ok button: ", err)
		}
		ui.IfSuccessThen(
			ui.WaitUntilExists(okButton),
			uiHandler.ClickUntil(
				okButton,
				ui.WithTimeout(2*time.Second).WaitUntilGone(okButton),
			),
		)
		return conn, nil
	}
	// Open Gvoice web.
	gConn, err := openGvoiceWeb(ctx)
	if err != nil {
		s.Fatal("Failed to open Google voice website", err)
	}
	defer gConn.Close()
	defer gConn.CloseTarget(ctx)

    if openGvoiceWeb == nil {
	    s.Fatal("Could not create new chrome tab")
	}

	// Login - Get 'Google Voice' tab
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()
	//gvoiceTab := nodewith.NameContaining("Google Voice").Role(role.RootWebArea)

    // find ok button to click and now enter email id
	//gvoiceTab := nodewith.NameContaining("Google Voice").Role(role.RootWebArea)
	gvoiceTab := nodewith.Role(role.RootWebArea)
    s.Log("goog voice tab: ", gvoiceTab)
    // Get text box and enter username
	textArea := nodewith.Role(role.TextField).Onscreen()
	if err := uiauto.Combine("Focus text field",
		ui.WaitUntilExists(textArea),
		ui.FocusAndWait(textArea),
		kb.AccelAction("Ctrl+A"),
		kb.AccelAction("Backspace"),
		kb.TypeAction(voiceusername),
	)(ctx); err != nil {
		s.Fatal("Failed to focus the text area: ", err)
	}

	nextButton := nodewith.Name("Next").Role(role.Button).First()
	//okButton := nodewith.Role(role.Button).Onscreen().First()
	if err := uiauto.Combine("Click on next button",
		ui.WaitUntilExists(nextButton),
		ui.FocusAndWait(nextButton),
		ui.LeftClick(nextButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click on next button: ", err)
	}

	// Get text box and enter password
	textArea = nodewith.Role(role.TextField).Onscreen()
	if err := uiauto.Combine("Focus text field",
		ui.WaitUntilExists(textArea),
		ui.FocusAndWait(textArea),
		kb.AccelAction("Ctrl+A"),
		kb.AccelAction("Backspace"),
		kb.TypeAction(voicepassword),
	)(ctx); err != nil {
		s.Fatal("Failed to focus the text area: ", err)
	}

	nextButton = nodewith.Name("Next").Role(role.Button).First()
	if err := uiauto.Combine("Click on next button",
		ui.WaitUntilExists(nextButton),
		ui.FocusAndWait(nextButton),
		ui.LeftClick(nextButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click on next button: ", err)
	}

	//userNameField := nodewith.NameContaining("Email or Phone").Role(role.TextField).Ancestor(gvoiceTab)
	/*userNameField := nodewith.NameContaining("Email or Phone").Role(role.TextField).Ancestor(textArea)
    s.Log("Email field:", userNameField)
	//userNameField := nodewith.Name("Username").Role(role.TextField).Ancestor(gvoiceTab)
	if err := uiauto.Combine("Enter user field",
			ui.LeftClick(userNameField),
			kb.AccelAction("Ctrl+A"),
			kb.AccelAction("Backspace"),
		    kb.TypeAction(voiceusername),
	)(ctx); err != nil {
		s.Fatal("Failed to enter userName: ", err)
	}
	s.Log("Email field:", userNameField)*/
	// Click Next button
	/*next := nodewith.Name("Next").Role(role.Button).Focusable().Ancestor(gvoiceTab)
	//nextButton := nodewith.Name("Next").Role(role.Button).Ancestor(gvoiceTab)
	if err := uiauto.Combine("Click on next button",
		ui.WaitUntilExists(next),
		ui.WithInterval(time.Second).LeftClickUntil(next, ui.Exists(gvoiceTab)),
		ui.LeftClick(next),
	)(ctx); err != nil {
		s.Fatal("Failed to click on next button: ", err)
	}*/
	//signInLink := nodewith.NameContaining("Sign in").Role(role.Link).Ancestor(msWebArea).First()
	// Click on + Send new message
	// Enter number on 'To'
	// Click Send to <number>
	// do two Tabs to reach 'Type a message'
	// paste message
	// Do click enter or send 'Enter' key

	// Check UI Notifications for SMS, verify sms content(normal, long, max)
	//if checkSMS(ctx)
    s.Log("xyz: waitig on notification")
	//_, err = ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(messageTitle))
	//if err != nil {
	//	s.Fatalf("Failed waiting %v for sms notification", uiTimeout)
	//}
	s.Log("xyz: Checking message")
	smsMessage := "Hello"
    // check message content
	//ui := uiauto.New(tconn).WithTimeout(60 * time.Second)

	notificationFinder := nodewith.Role(role.StaticText).Name(smsMessage)
    s.Log("xyz: clearing on notification.", notificationFinder)
	// Clear message for now. Should be clear notification
	if err := ui.LeftClick(notificationFinder)(ctx); err != nil {
		s.Fatal("Failed finding notification and clicking it: ", err)
	}

}

//func checkSMS(ctx context.Context) (err error) {
//	return err
//}
