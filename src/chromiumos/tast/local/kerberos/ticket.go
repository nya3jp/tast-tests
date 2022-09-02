// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kerberos contains details about Kerberos setup that is used in
// testing.
package kerberos

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// AddTicket adds a kerberos ticket via the settings ui.
func AddTicket(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, ui *uiauto.Context, keyboard *input.KeyboardEventWriter, config *Configuration, password string) error {

	_, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/kerberos")
	if err != nil {
		return errors.Wrap(err, "could not open kerberos section in OS settings")
	}

	// Add a Kerberos ticket.
	if err := uiauto.Combine("add Kerberos ticket",
		ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link)),
		ui.LeftClick(nodewith.Name("Add a ticket").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Kerberos username").Role(role.TextField)),
		keyboard.TypeAction(config.KerberosAccount),
		ui.LeftClick(nodewith.Name("Password").Role(role.TextField)),
		keyboard.TypeAction(password),
		ui.LeftClick(nodewith.Name("Advanced").Role(role.Link)),
		ui.LeftClick(nodewith.Role(role.TextField).State(state.Editable, true).State(state.Multiline, true)),
		keyboard.TypeAction(config.RealmsConfig),
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Add").HasClass("action-button")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to add Kerberos ticket")
	}

	if err := CheckForTicket(ctx, ui, config); err != nil {
		return errors.Wrap(err, "failed to find active ticket")
	}

	apps.Close(ctx, tconn, apps.Settings.ID)

	// Wait for OS Setting to close.
	if err := ui.WaitUntilGone(nodewith.Name("Settings - Kerberos tickets").Role(role.Window))(ctx); err != nil {
		return errors.Wrap(err, "failed to close os settings")
	}

	return nil
}

// CheckForTicket tries to find the ticket in the kerberos menu and checks that it's not "expired".
func CheckForTicket(ctx context.Context, ui *uiauto.Context, config *Configuration) error {
	// Wait for ticket to appear.
	testing.ContextLog(ctx, "Waiting for Kerberos ticket to appear")
	if err := ui.WaitUntilExists(nodewith.Name(config.KerberosAccount).Role(role.StaticText).State(state.Editable, false))(ctx); err != nil {
		return errors.Wrap(err, "failed to find Kerberos ticket")
	}

	// Check that ticket is "Active".
	if err := ui.WaitUntilExists(nodewith.Name("Active").Role(role.StaticText))(ctx); err != nil {
		return errors.Wrap(err, "Kerberos ticket is not Active")
	}

	return nil
}

// ClickAdvancedAndProceed presses "Advanced" and "Proceed" buttons on the
// Kerberos test website to accept the warning. This is needed because the
// website lacks security certificate.
func ClickAdvancedAndProceed(ctx context.Context, conn *chrome.Conn) error {
	clickAdvance := fmt.Sprintf("document.getElementById(%q).click()", "details-button")
	clickProceed := fmt.Sprintf("document.getElementById(%q).click()", "proceed-link")

	// Trying to click "Advance" button until success or timeout.
	// Returns error in case of failure.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, clickAdvance, nil); err != nil {
			return errors.Wrap(err, "failed to click Advance button")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return err
	}

	// Trying to click "Proceed" button until success or timeout.
	// Returns error in case of failure.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, clickProceed, nil); err != nil {
			return errors.Wrap(err, "failed to click Proceed button")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return err
	}

	// Wait for webpage to load its content after we accepted the warning.
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "failed to wait for the URL to load")
	}

	return nil
}
