// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kerberos contains details about Kerberos setup that is used in
// testing.
package kerberos

import (
	"context"

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

	// TODO: chromium/1249773 change to get "Active" state once the bug is
	// resolved. UI tree is not refreshed for 1 minute.
	// Check that ticket is not expired.
	if err := ui.Exists(nodewith.Name("Expired").Role(role.StaticText))(ctx); err == nil {
		return errors.New("Kerberos ticket has expired")
	}

	return nil
}
