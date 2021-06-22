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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
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
func SettingsPage(ctx context.Context, cr *chrome.Chrome, shortLink string) *openedPage {
	page := &openedPage{
		cr: cr,
	}

	conn, err := cr.NewConn(ctx, "chrome://settings/"+shortLink)
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

// CheckNodeAttributes returns whether this node matches the given expected params.
// It returns an error with the first non-matching attribute, nil otherwise.
func CheckNodeAttributes(n *ui.Node, expectedParams ui.FindParams) error {
	if expectedRestriction, exist := expectedParams.Attributes["restriction"]; exist {
		if n.Restriction != expectedRestriction {
			return errors.Errorf("unexpected restriction: got %#v; want %#v", n.Restriction, expectedRestriction)
		}
	}
	if expectedChecked, exist := expectedParams.Attributes["checked"]; exist {
		if n.Checked != expectedChecked {
			return errors.Errorf("unexpected checked: got %#v; want %#v", n.Checked, expectedChecked)
		}
	}
	if expectedName, exist := expectedParams.Attributes["name"]; exist {
		if n.Name != expectedName {
			return errors.Errorf("unexpected name: got %#v; want %#v", n.Name, expectedName)
		}
	}
	return nil
}

// VerifySettingsNode finds a node with the given params.
// It also compares the attributes of the found node against the given expected params.
func VerifySettingsNode(ctx context.Context, tconn *chrome.TestConn, params, expectedParams ui.FindParams) error {
	node, err := ui.FindWithTimeout(ctx, tconn, params, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "finding the node failed")
	}
	defer node.Release(ctx)
	return CheckNodeAttributes(node, expectedParams)
}

// VerifySettingsState opens a settings page with given link (e.g. "chrome://settings/content/location").
// Then it finds a node with the given params.
// It also compares the attributes of the found node with the given expected params.
func VerifySettingsState(ctx context.Context, cr *chrome.Chrome, settingsPage string, params, expectedParams ui.FindParams) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	conn, err := cr.NewConn(ctx, settingsPage)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the settings page")
	}
	defer conn.Close()
	return VerifySettingsNode(ctx, tconn, params, expectedParams)
}

// VerifyOSSettingsState opens a OS settings page with given link (e.g. "chrome://os-settings/device/display").
// Then it finds a node with the given params.
// It alsos compare the attributes of the found node with the given expected params.
func VerifyOSSettingsState(ctx context.Context, cr *chrome.Chrome, osSettingsPage string, params, expectedParams ui.FindParams) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	conn, err := apps.LaunchOSSettings(ctx, cr, osSettingsPage)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the OS Settings page")
	}
	defer conn.Close()
	return VerifySettingsNode(ctx, tconn, params, expectedParams)
}
