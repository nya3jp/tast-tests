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

type tabFindParams string

func (t tabFindParams) uiFindParams() ui.FindParams {
	return ui.FindParams{
		Name: string(t),
		Role: ui.RoleTypeTreeItem,
	}
}

// Tab names in Help app.
const (
	SearchTab   = tabFindParams("Search")
	OverviewTab = tabFindParams("Overview")
	PerksTab    = tabFindParams("Perks")
	HelpTab     = tabFindParams("Help")
	WhatsNewTab = tabFindParams("What’s new")
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
	if _, err := helpRootNode.DescendantWithTimeout(ctx, OverviewTab.uiFindParams(), 20*time.Second); err != nil {
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
	if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
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
// https://source.chromium.org/chromium/chromium/src/+/HEAD:chrome/browser/chromeos/web_applications/chrome_help_app_ui_delegate.cc;l=53;drc=c2c84a5ac7711dedcc0b7ff9e79bf7f2da019537.
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

// IsTabShown checks whether tab is shown.
func IsTabShown(ctx context.Context, tconn *chrome.TestConn, tabParams tabFindParams) (bool, error) {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to find help app")
	}
	defer helpRootNode.Release(ctx)

	return helpRootNode.DescendantExists(ctx, tabParams.uiFindParams())
}

// ClickTab clicks the tab with given name.
func ClickTab(ctx context.Context, tconn *chrome.TestConn, tabParams tabFindParams) error {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find help app")
	}
	defer helpRootNode.Release(ctx)

	tabNode, err := helpRootNode.DescendantWithTimeout(ctx, tabParams.uiFindParams(), 20*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find tab node with %v", tabParams.uiFindParams())
	}
	defer tabNode.Release(ctx)

	return tabNode.StableLeftClick(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second})
}

// HelpRootNode returns the root ui node of Help app.
func HelpRootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	return ui.FindWithTimeout(ctx, tconn, helpRootNodeParams, 20*time.Second)
}

// WaitWhatsNewTabRendered waits for What's New tab to be rendered.
// The large text at the top of the page seems like a natural choice since it's easily
// recognizable and unlikely to change frequently. It would be better to have a
// successful launch indicator that doesn't rely on a string, though.
func WaitWhatsNewTabRendered(ctx context.Context, tconn *chrome.TestConn) error {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find help app")
	}
	defer helpRootNode.Release(ctx)

	// Particularly in this case, the apostrophe in What’s is not actually the normal
	// apostrophe character, but instead the "right single quotation mark" character (&rsquo;).
	titleParams := ui.FindParams{Role: ui.RoleTypeStaticText, Name: "What's new with Chromebook?"}
	return ui.WaitUntilExists(ctx, tconn, titleParams, 30*time.Second)
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

// DescendantsWithTimeout returns all nodes in help app matching params.
// It waits for the first element appear and returns all findings immediately.
// Thus, this function can not be used when elements are shown up one by one.
func DescendantsWithTimeout(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, timeout time.Duration) (ui.NodeSlice, error) {
	helpRootNode, err := HelpRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find help app")
	}
	defer helpRootNode.Release(ctx)

	if err := helpRootNode.WaitUntilDescendantExists(ctx, params, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to find help app")
	}

	return helpRootNode.Descendants(ctx, params)
}
