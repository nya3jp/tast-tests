// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package helpapp contains common functions used in the help app.
package helpapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

var helpRootNodeParams = ui.FindParams{
	Name: apps.Help.Name,
	Role: ui.RoleTypeRootWebArea,
}

// WaitForApp waits for the app to be shown and rendered.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn) error {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find help app")
	}
	// Find Overview tab to verify app rendering.
	params := ui.FindParams{
		Name: "Overview",
		Role: ui.RoleTypeTreeItem,
	}
	if _, err := helpRootNode.DescendantWithTimeout(ctx, params, 20*time.Second); err != nil {
		return errors.Wrap(err, "failed to render help app")
	}
	return nil
}

// Launch launches help app and waits for it to be present in shelf.
func Launch(ctx context.Context, tconn *chrome.TestConn) error {
	app := apps.Help
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		return errors.Wrapf(err, "failed to launch %s", app.Name)
	}

	testing.ContextLog(ctx, "Wait for help app shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
	}
	return nil
}

// Exists checks whether the help app exists in the accessiblity tree.
func Exists(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return ui.Exists(ctx, tconn, helpRootNodeParams)
}

// LoadTimeData struct for the help app.
// Following fields populated by |ChromeHelpAppUIDelegate::PopulateLoadTimeData|
// https://source.chromium.org/chromium/chromium/src/+/master:chrome/browser/chromeos/web_applications/chrome_help_app_ui_delegate.cc;l=53;drc=c2c84a5ac7711dedcc0b7ff9e79bf7f2da019537.
type LoadTimeData struct {
	IsManagedDevice bool `json:"isManagedDevice"`
}

// GetLoadTimeData returns some of the LoadTimeData fields from the help app.
func GetLoadTimeData(ctx context.Context, cr *chrome.Chrome) (*LoadTimeData, error) {
	// Establish a Chrome connection to the Help app and wait for it to finish loading.
	helpAppConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome-untrusted://help-app/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection to help app")
	}
	defer helpAppConn.Close()

	if err := helpAppConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "failed to wait for help app to finish loading")
	}
	data := &LoadTimeData{}
	if err := helpAppConn.Eval(ctx, "window.loadTimeData.data_", &data); err != nil {
		return nil, errors.Wrap(err, "failed to evaluate window.loadTimeData.data_")
	}
	return data, nil
}

// IsPerkShown checks if the perks tab is displayed or not.
func IsPerkShown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return isTabShown(ctx, tconn, "Perks")
}

func isTabShown(ctx context.Context, tconn *chrome.TestConn, tabName string) (bool, error) {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to find help app")
	}

	params := ui.FindParams{
		Name: tabName,
		Role: ui.RoleTypeTreeItem,
	}
	return helpRootNode.DescendantExists(ctx, params)
}

// HelpRootNode returns the root ui node of Help app.
func HelpRootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	return ui.FindWithTimeout(ctx, tconn, helpRootNodeParams, 20*time.Second)
}

// LaunchFromThreeDotMenu launches Help app from three dot menu.
func LaunchFromThreeDotMenu(ctx context.Context, tconn *chrome.TestConn) error {
	clickElement := func(params ui.FindParams) error {
		element, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
		if err != nil {
			return errors.Wrapf(err, "failed to find element with %v", params)
		}
		defer element.Release(ctx)
		if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for location change completed")
		}
		return element.LeftClick(ctx)
	}

	// Find and click the three dot menu via UI.
	if err := clickElement(ui.FindParams{
		Role:      ui.RoleTypePopUpButton,
		ClassName: "BrowserAppMenuButton",
	}); err != nil {
		return errors.Wrap(err, "failed to click three dot menu")
	}

	// Find and click Help in three dot menu via UI.
	if err := clickElement(ui.FindParams{
		Name:      "Help",
		ClassName: "MenuItemView",
	}); err != nil {
		return errors.Wrap(err, "failed to click help in three dot menu")
	}

	// Find and click Get Help in three dot menu via UI.
	if err := clickElement(ui.FindParams{
		Name:      "Get Help",
		ClassName: "MenuItemView",
	}); err != nil {
		return errors.Wrap(err, "failed to click help in three dot menu")
	}

	return WaitForApp(ctx, tconn)
}
