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

// IsPerkShown checks if the perks tab is displayed or not.
func IsPerkShown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return isTabShown(ctx, tconn, "Perks")
}

// IsWhatsnewShown checks if the perks tab is displayed or not.
func IsWhatsnewShown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return isTabShown(ctx, tconn, "What's new")
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

// LaunchFromThreeDotMenu launchs Help app from three dot menu
func LaunchFromThreeDotMenu(ctx context.Context, tconn *chrome.TestConn) error {
	// Find and click the three dot menu via UI.
	params := ui.FindParams{
		Role:      ui.RoleTypePopUpButton,
		ClassName: "BrowserAppMenuButton",
	}
	menu, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the three dot menu")
	}
	defer menu.Release(ctx)

	if err := menu.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click three dot menu")
	}

	// Find and click Help in three dot menu via UI.
	helpMenuParams := ui.FindParams{
		Name:      "Help",
		ClassName: "MenuItemView",
	}
	helpMenu, err := ui.FindWithTimeout(ctx, tconn, helpMenuParams, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find Help in three dot menu")
	}
	defer menu.Release(ctx)

	if err := helpMenu.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click help in three dot menu")
	}

	// Find and click Help in three dot menu via UI.
	getHelpMenuParams := ui.FindParams{
		Name:      "Get Help",
		ClassName: "MenuItemView",
	}
	getHelpMenu, err := ui.FindWithTimeout(ctx, tconn, getHelpMenuParams, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find get Help in help sub-menu")
	}
	defer menu.Release(ctx)

	if err := getHelpMenu.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click get Help in help sub-menu")
	}

	return WaitForApp(ctx, tconn)
}
