// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package citrix

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/testing"
)

// PWAConnector structure used for performing operation on Citrix app.
type PWAConnector struct {
	ApplicationID string
	dataPath      func(string) string
	detector      *uidetection.Context
	tconn         *chrome.TestConn
}

// Init initializes state of the connector.
func (pc *PWAConnector) Init(s *testing.FixtState, tconn *chrome.TestConn, d *uidetection.Context) {
	pc.dataPath = s.DataPath
	pc.detector = d
	pc.tconn = tconn
}

// Login connects to the server and logs in using information provided in config.
func (pc *PWAConnector) Login(ctx context.Context, k *input.KeyboardEventWriter, cfg *apps.VDILoginConfig) error {

	if err := pc.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.CustomIcon(pc.dataPath(CitrixData[SplashscreenServerURLTbx])))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for Citrix splashscreen")
	}

	if err := pc.tconn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "failed waiting for URL to load")
	}

	testing.ContextLog(ctx, "Citrix: logging in")
	if err := k.Type(ctx, cfg.Username); err != nil {
		return errors.Wrap(err, "failed to enter user name")
	}
	if err := k.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "failed to execute Tab")
	}
	if err := k.Type(ctx, cfg.Password); err != nil {
		return errors.Wrap(err, "failed to enter password")
	}
	if err := k.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to execute Enter")
	}

	return nil
}

// WaitForMainScreenVisible ensures that element visible on the screen
// indicates it is the main Citrix screen.
func (pc *PWAConnector) WaitForMainScreenVisible(ctx context.Context) error {
	if err := pc.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"Search", "Workspace"}))(ctx); err != nil {
		return errors.Wrap(err, "didn't see expected text block after logging into Citrix")
	}
	return nil
}

// SearchAndOpenApplication opens given application using search provided in
// Citrix, runs checkIfOpened function to ensure app opened. Before calling
// make sure main Citrix screen is visible by calling
// WaitForMainScreenVisible(). Call ResetSearch() to clean the search state.
func (pc *PWAConnector) SearchAndOpenApplication(ctx context.Context, k *input.KeyboardEventWriter, appName string, checkIfOpened func(context.Context) error) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Citrix: opening app %s", appName)
		return uiauto.Combine("open application "+appName+" in PWA Citrix",
			k.AccelAction("Tab"),
			k.AccelAction("Tab"),
			k.AccelAction("Tab"),   // Go to the search field.
			k.AccelAction("Enter"), // Move focus to the search field.
			k.TypeAction(appName),
			k.AccelAction("Down"),  // Go to the first result.
			k.AccelAction("Enter"), // Open the app.
			checkIfOpened,
		)(ctx)
	}
}

// ResetSearch cleans search field. Call only when search was triggered by
// SearchAndOpenApplication().
func (pc *PWAConnector) ResetSearch(ctx context.Context, k *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Citrix: cleaning search")
	// Reloading the page will bring the initial state of the application.
	if err := k.Accel(ctx, "Ctrl + r"); err != nil {
		return errors.Wrap(err, "failed to execute Ctrl+r")
	}
	return nil
}
