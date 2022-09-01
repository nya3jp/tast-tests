// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// nodeChecker is used in checking different properties of a node while collecting the error messages.
type nodeChecker struct {
	err  error
	info *uiauto.NodeInfo
}

// openedPage stores information that allows chaining SettingsPage() and SelectNode() together
// without the repetition of some input parameters.
type openedPage struct {
	cr  *chrome.Chrome
	err error
}

// SettingsPage opens a settings page with given link (e.g. "content/location" -> "chrome://settings/content/location").
// The returned openedPage value can be used to select a node from the node tree (not just from the page).
func SettingsPage(ctx context.Context, cr *chrome.Chrome, br ash.ConnSource, shortLink string) *openedPage {
	page := &openedPage{
		cr: cr,
	}

	conn, err := br.NewConn(ctx, "chrome://settings/"+shortLink)
	if err != nil {
		page.err = err
		return page
	}
	defer conn.Close()

	return page
}

// OSSettingsPage opens the OS settings page with given link (e.g. "osAccessibility" -> "chrome://os-settings/osAccessibility").
// The returned openedPage value can be used to select a node from the node tree (not just from the page).
func OSSettingsPage(ctx context.Context, cr *chrome.Chrome, shortLink string) *openedPage {
	page := &openedPage{
		cr: cr,
	}

	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/"+shortLink)
	if err != nil {
		page.err = err
		return page
	}
	defer conn.Close()

	return page
}

// OSSettingsPageWithPassword open the OS settings page with given link (e.g. "osAccessibility" -> "chrome://os-settings/osAccessibility").
// If the opened settings page is password protected, try to authenticate with the given password.
// The returned openedPage value can be used to select a node from the node tree (not just from the page).
func OSSettingsPageWithPassword(ctx context.Context, cr *chrome.Chrome, shortLink, password string) *openedPage {
	page := &openedPage{
		cr: cr,
	}

	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/"+shortLink)
	if err != nil {
		page.err = errors.Wrap(err, "failed to launch OS Settings")
		return page
	}
	defer conn.Close()

	tconn, err := page.cr.TestAPIConn(ctx)
	if err != nil {
		page.err = errors.Wrap(err, "failed to create Test API connection")
		return page
	}

	passwordNode := nodewith.Name("Confirm your password")

	uia := uiauto.New(tconn)
	if err := uia.WaitUntilExists(passwordNode.First())(ctx); err != nil {
		testing.ContextLog(ctx, "Could not find password dialog: ", err)
		return page
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		page.err = errors.Wrap(err, "failed to open keyboard device")
		return page
	}
	defer keyboard.Close()

	if err := keyboard.Type(ctx, password+"\n"); err != nil {
		page.err = errors.Wrap(err, "failed to type password")
		return page
	}

	if err := uia.WaitUntilGone(passwordNode)(ctx); err != nil {
		testing.ContextLog(ctx, "Could not wait until password dialog is gone: ", err)
		return page
	}

	return page
}

// CurrentPage return with an openedPage, which can be used to select a node from the node tree (not just from the page).
func CurrentPage(cr *chrome.Chrome) *openedPage {
	page := &openedPage{
		cr: cr,
	}

	return page
}

// SelectNode creates a nodeChecker from the selected node.
// It can be used to verify different properties of the node.
func (page *openedPage) SelectNode(ctx context.Context, finder *nodewith.Finder) *nodeChecker {
	checker := &nodeChecker{}

	if page.err != nil {
		checker.err = errors.Wrap(page.err, "unable to select node as page was not opened properly")
		return checker
	}

	tconn, err := page.cr.TestAPIConn(ctx)
	if err != nil {
		checker.err = errors.Wrap(err, "failed to create Test API connection")
		return checker
	}

	uia := uiauto.New(tconn)
	info, err := uia.Info(ctx, finder)
	if err != nil {
		checker.err = errors.Wrap(err, "failed get the info of the node")
		return checker
	}

	checker.info = info
	return checker
}

// Restriction checks the restriction state of a Settings node given by the SettingsState() function.
func (checker *nodeChecker) Restriction(expectedRestriction restriction.Restriction) *nodeChecker {
	if checker.err != nil || checker.info == nil {
		return checker
	}

	if checker.info.Restriction != expectedRestriction {
		checker.err = errors.Errorf("unexpected node restriction state; want %q, got %q", expectedRestriction, checker.info.Restriction)
	}

	return checker
}

// Checked checks the checked state of a Settings node given by the SettingsState() function.
func (checker *nodeChecker) Checked(expectedChecked checked.Checked) *nodeChecker {
	if checker.err != nil || checker.info == nil {
		return checker
	}

	if checker.info.Checked != expectedChecked {
		checker.err = errors.Errorf("unexpected node checked state; want %q, got %q", expectedChecked, checker.info.Checked)
	}

	return checker
}

// Verify is the last element of the Settings state verifying.
// It returns with the error collected during the process.
func (checker *nodeChecker) Verify() error {
	return checker.err
}

// CheckCertificateVisibleInSystemSettings checks that a certificate with the
// given name is usable for configuring a Wi-Fi connection. Fully creating such
// a connection would require a special network environment, so it just tests
// that the certificate is visible and selectable.
func CheckCertificateVisibleInSystemSettings(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, certName string) error {
	OSSettingsPage(ctx, cr, "network")
	ui := uiauto.New(tconn)

	if err := ui.LeftClick(nodewith.Name("Add network connection").Role(role.Button))(ctx); err != nil {
		return err
	}

	err := ui.WithTimeout(time.Second).WaitUntilExists(nodewith.Name("Add Wi-Fi…").Role(role.Button))(ctx)
	if err != nil {
		if err := ui.LeftClick(nodewith.Name("Wi-Fi enable").Role(role.ToggleButton))(ctx); err != nil {
			return err
		}
	}

	return uiauto.Combine("use system settings",
		ui.WaitUntilExists(nodewith.Name("Add Wi-Fi…").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Add Wi-Fi…").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Security").ClassName("md-select")),
		ui.LeftClick(nodewith.Name("EAP").Role(role.ListBoxOption)),
		ui.WaitUntilExists(nodewith.Name("EAP method").ClassName("md-select")),
		ui.LeftClick(nodewith.Name("EAP method").ClassName("md-select")),
		ui.LeftClick(nodewith.Name("EAP-TLS").Role(role.ListBoxOption)),
		ui.WaitUntilExists(nodewith.Name("Server CA certificate").Role(role.InlineTextBox)),
		ui.MakeVisible(nodewith.Name("User certificate").Role(role.InlineTextBox)),
		ui.LeftClick(nodewith.Name("User certificate").ClassName("md-select")),
		ui.LeftClick(nodewith.Name(certName+" ["+certName+"]").Role(role.ListBoxOption)))(ctx)
}
