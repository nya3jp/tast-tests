// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package helpapp contains common functions used in the help app.
package helpapp

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// Tab names in Help app
const (
	OverviewTab = "Overview"
	PerksTab    = "Perks"
	HelpTab     = "Help"
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
	if _, err := helpRootNode.DescendantWithTimeout(ctx, tabFindParams(OverviewTab), 20*time.Second); err != nil {
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

// IsTabShown checks if the tab is shown or not.
func IsTabShown(ctx context.Context, tconn *chrome.TestConn, tabName string) (bool, error) {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to find help app")
	}
	defer helpRootNode.Release(ctx)

	return helpRootNode.DescendantExists(ctx, tabFindParams(tabName))
}

// ClickTab clicks the tab with given name
func ClickTab(ctx context.Context, tconn *chrome.TestConn, tabName string) error {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find help app")
	}
	defer helpRootNode.Release(ctx)

	tabNode, err := helpRootNode.DescendantWithTimeout(ctx, tabFindParams(tabName), 20*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find %s tab node", tabName)
	}
	defer tabNode.Release(ctx)

	return tabNode.LeftClick(ctx)
}

func tabFindParams(tabName string) ui.FindParams {
	return ui.FindParams{
		Name: tabName,
		Role: ui.RoleTypeTreeItem,
	}
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

// DescendantWithTimeout finds a node in help app using params and returns it.
func DescendantWithTimeout(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, timeout time.Duration) (*ui.Node, error) {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find help app")
	}
	defer helpRootNode.Release(ctx)
	return helpRootNode.DescendantWithTimeout(ctx, params, timeout)
}

// UIConn returns a connection to the Help app,
// where JavaScript can be executed to simulate interactions with the UI.
func UIConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	f := func(t *target.Info) bool { return t.Title == "Explore" }
	return c.NewConnForTarget(ctx, f)
}
