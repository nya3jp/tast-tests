// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package citrix

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/testing"
)

// Connector structure used for performing operation on Citrix app.
type Connector struct {
	ApplicationID string
	state         *testing.FixtState
	detector      *uidetection.Context
}

// Init initializes state od the connector.
func (c *Connector) Init(s *testing.FixtState, d *uidetection.Context) {
	c.state = s
	c.detector = d
}

// Login connects to the server and logs in using information provided in
// config.
func (c *Connector) Login(ctx context.Context, k *input.KeyboardEventWriter, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "Logging into Citrix")

	if err := c.detector.WaitUntilExists(uidetection.CustomIcon(c.state.DataPath(CitrixData[SplashscreenServerURLTbx])))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for Citrix splashscreen")
	}

	testing.ContextLog(ctx, "Enter server url")
	if err := uiauto.Combine("enter citrix server url, connect and wait for next screen",
		k.AccelAction("Tab"), // Enter server test box.
		k.TypeAction(cfg.Server),
		k.AccelAction("Enter"), // Connect to the server.
		c.detector.WaitUntilExists(uidetection.TextBlock([]string{"User", "name"})),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering server url")
	}

	testing.ContextLog(ctx, "Enter username and password")
	if err := uiauto.Combine("enter username and password and connect login",
		k.TypeAction(cfg.Username),
		k.AccelAction("Tab"),
		k.TypeAction(cfg.Password),
		k.AccelAction("Tab"),
		k.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering username or password")
	}

	return nil
}

// WaitForMainScreenVisible ensures that element visible on the screen
// indicates it is the main Citrix screen.
func (c *Connector) WaitForMainScreenVisible(ctx context.Context) error {
	if err := c.detector.WaitUntilExists(uidetection.TextBlock([]string{"Search", "Workspace"}))(ctx); err != nil {
		return errors.Wrap(err, "didn't see expected text block after logging into Citrix")
	}
	return nil
}

// SearchAndOpenApplication opens given application using search provided in
// Citrix. Before calling make sure main Citrix screen is visible by calling
// WaitForMainScreenVisible(). all ResetSearch() to clean the search state.
func (c *Connector) SearchAndOpenApplication(ctx context.Context, k *input.KeyboardEventWriter, appName string) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Citrix - opening app %s", appName)
		return uiauto.Combine("open application "+appName+" in Citrix",
			// c.detector.LeftClick(uidetection.TextBlock([]string{"Search", "Workspace"})),// It wound found it but click does not work for some reason.
			k.AccelAction("Tab"),
			k.AccelAction("Tab"),
			k.AccelAction("Tab"),
			k.AccelAction("Tab"),
			k.AccelAction("Tab"),   // Go to the search field.
			k.AccelAction("Enter"), // Move focus to the search field.
			k.TypeAction(appName),
			k.AccelAction("Down"),  // Go to the first result.
			k.AccelAction("Enter"), // Open the app.
		)(ctx)
	}
}

// ResetSearch cleans search field. Call only when search was triggered by
// SearchAndOpenApplication().
func (c *Connector) ResetSearch(ctx context.Context, k *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Citrix - clean search")
	// Check that result was actually triggered.
	if err := c.detector.WaitUntilExists(uidetection.TextBlock([]string{"See", "more", "results"}))(ctx); err != nil {
		return errors.Wrap(err, "search results view is not visible")
	}
	// Citrix, after executed search, opened and closed app keeps focus on the
	// search field.
	if err := uiauto.Combine("clearing search results",
		k.AccelAction("Ctrl+a"),    // Select search phrase,
		k.AccelAction("Backspace"), // Clear the selection.
		k.AccelAction("Esc"),       // Return to the Citrix view.
	)(ctx); err != nil {
		return errors.Wrap(err, "couldn't clear the search results")
	}
	return nil
}
