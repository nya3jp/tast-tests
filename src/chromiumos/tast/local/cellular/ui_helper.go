// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cellular provides functions for testing Cellular connectivity.
package cellular

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// UIHelper creates chrome target and testapi connectoin.
type UIHelper struct {
	UIHandler cuj.UIActionHandler
	UI        *uiauto.Context
	Cr        *chrome.Chrome
	Tconn     *chrome.TestConn
}

const (
	messageClear      = "Clear all"
	gvoiceMessagesURL = "https://voice.google.com/u/0/messages"
)

// NewUIHelper creates a Helper object and ensures that a UI is loaded.
func NewUIHelper(ctx context.Context, username, password string) (*UIHelper, error) {

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		//chrome.GAIALogin(chrome.Creds{User: s.RequiredVar("ui.oac_username"), Pass: s.RequiredVar("ui.oac_password")}),
		//chrome.GAIALogin(chrome.Creds{User: "JorahStark.814141@gmail.com", Pass: "zeFpqzqnW_dchFh"}),
		chrome.ProdPolicy(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "chrome login failed")
	}

	creds := cr.Creds()
	testing.ContextLogf(ctx, "test api conn: %s", creds)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect test api")
	}

	ui := uiauto.New(tconn)

	// uiHandler will be assigned with different instances for clamshell and tablet mode.
	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create clamshell action handler")
	}

	helper := UIHelper{UIHandler: uiHandler, UI: ui, Cr: cr, Tconn: tconn}
	return &helper, nil
}

// GoogleVoiceLogin attempts to login on google voice message page.
func (h *UIHelper) GoogleVoiceLogin(ctx context.Context) (time.Duration, *chrome.Conn, error) {
	ctx, st := timing.Start(ctx, "UIHelper.GoogleVoiceLogin")
	defer st.End()

	start := time.Now()

	testing.ContextLog(ctx, "open googlevoice web url: ", gvoiceMessagesURL)
	// Open google voice tab, set new window to true to be first tab
	driverconn, err := h.UIHandler.NewChromeTab(ctx, h.Cr.Browser(), gvoiceMessagesURL, true)
	if err != nil {
		return time.Since(start), driverconn, errors.Wrap(err, "failed to open voice web page")
	}

	if err := webutil.WaitForRender(ctx, driverconn, 2*time.Minute); err != nil {
		return time.Since(start), driverconn, errors.Wrap(err, "failed to wait for render to finish")
	}

	if err := webutil.WaitForQuiescence(ctx, driverconn, 2*time.Minute); err != nil {
		return time.Since(start), driverconn, errors.Wrap(err, "failed to wait for voice page to finish loading")
	}

	// Google Voice sometimes pops up a prompt to notice about notifications,
	// asking to allow/dismiss notifications.
	allowPrompt := nodewith.Name("Allow").Role(role.Button).Onscreen()

	if err := uiauto.Combine("Click on allow button",
		h.UI.WaitUntilExists(allowPrompt),
		h.UI.FocusAndWait(allowPrompt),
		h.UI.LeftClick(allowPrompt),
	)(ctx); err != nil {
		return time.Since(start), driverconn, errors.Wrap(err, "failed to click on allow button")
	}

	return time.Since(start), driverconn, nil
}

// SendMessage - sends sms to dut mobile number using google voice with ota.
func (h *UIHelper) SendMessage(ctx context.Context, number, message string) (time.Duration, error) {
	ctx, st := timing.Start(ctx, "UIHelper.SendMessage")
	defer st.End()

	start := time.Now()

	// Keyboard to input key inputs.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return time.Since(start), errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// Click on + Send new message.
	sendButton := nodewith.NameContaining("Send new message").Role(role.Button)
	if err := uiauto.Combine("Click on send new message button",
		h.UI.WaitUntilExists(sendButton),
		h.UI.LeftClick(sendButton),
	)(ctx); err != nil {
		return time.Since(start), errors.Wrap(err, "failed to click on + button")
	}

	testing.ContextLog(ctx, "selected send new message button")
	// Enter number on 'To'.
	textArea := nodewith.Name("To").Role(role.InlineTextBox).Onscreen()
	textAreaFocused := textArea.Focused()
	h.UIHandler.ClickUntil(textArea, h.UI.Exists(textAreaFocused))
	if err := uiauto.Combine("Focus number text field",
		h.UI.WaitUntilExists(textArea),
		kb.AccelAction("Ctrl+A"),
		kb.AccelAction("Backspace"),
		kb.TypeAction(number),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		return time.Since(start), errors.Wrap(err, "failed to focus the phone number text field")
	}

	testing.ContextLog(ctx, "enter text in type a message textfield")
	smsTextField := nodewith.Name("Type a message").Role(role.TextField).State(state.Editable, true).Onscreen()
	smsTextFieldFocused := smsTextField.Focused()
	info, err := h.UI.Info(ctx, smsTextField)
	if err != nil {
		return time.Since(start), errors.Wrap(err, "error reading smstextfield")
	}
	if info.State[state.Invisible] {
		testing.ContextLog(ctx, "sms text field invisible")
		if err := kb.Accel(ctx, "Shift+Tab+Enter"); err != nil {
			return time.Since(start), errors.Wrap(err, "failed to press shift+tab+enter to close consent banner")
		}
	}

	// Get text box and enter mobile number.
	h.UIHandler.ClickUntil(smsTextField, h.UI.Exists(smsTextFieldFocused))
	if err := uiauto.Combine("Focus message text field",
		h.UI.WaitUntilExists(smsTextField.Visible()),
		h.UI.FocusAndWait(smsTextField),
		h.UI.LeftClick(smsTextField),
		kb.TypeAction(message),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		return time.Since(start), errors.Wrap(err, "failed to focus the message text area")
	}

	testing.ContextLog(ctx, "click on send button")
	// Do click enter or send 'Enter' key.
	sendButton = nodewith.Name("Send message").Role(role.Button)
	if err := uiauto.Combine("Click on send button",
		h.UI.WaitUntilExists(sendButton),
		h.UI.LeftClick(sendButton),
	)(ctx); err != nil {
		return time.Since(start), errors.Wrap(err, "failed to click on send button")
	}

	return time.Since(start), nil
}

// ValidateMessage - validates sms received.
func (h *UIHelper) ValidateMessage(ctx context.Context, messageSent, phno string) (time.Duration, error) {
	ctx, st := timing.Start(ctx, "UIHelper.ValidateMessage")
	defer st.End()

	start := time.Now()

	// check message content.
	notification := nodewith.Role(role.Window).ClassName("ash/message_center/MessagePopup")

	// check notification window.
	if err := h.UI.WaitUntilExists(notification)(ctx); err != nil {
		return time.Since(start), errors.Wrap(err, "failed to see sms notification dialog")
	}

	testing.ContextLog(ctx, "notification details:", notification)
	alertDialog := nodewith.Role(role.AlertDialog).ClassName("MessagePopupView").Onscreen()

	// Read number and sms message and compare and close dialog.
	smsDetails, err := h.UI.Info(ctx, alertDialog)
	if err != nil {
		return time.Since(start), errors.Wrap(err, "failed to see sms notification dialog content")
	}

	testing.ContextLog(ctx, "alert dialog data: ", smsDetails)

	//message := strings.SplitAfter(smsDetails.Name, "+16505494199")
	//phone no in dut : 918317507407
	//message := strings.SplitAfter(smsDetails.Name, "+917672078596")
	message := strings.SplitAfter(smsDetails.Name, "+"+phno)

	if strings.Contains(message[1], messageSent) {
		testing.ContextLog(ctx, "success message received")
		return time.Since(start), nil
	}

	return time.Since(start), errors.Wrap(err, "failed to receive sms")
}
