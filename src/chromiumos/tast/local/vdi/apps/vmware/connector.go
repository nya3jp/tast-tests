// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmware

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/testing"
)

// Connector structure used for performing operation on Vmware app.
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
	testing.ContextLog(ctx, "Logging into Vmware")

	if err := c.detector.WaitUntilExists(uidetection.CustomIcon(c.state.DataPath(UIFragments[SplashscreenAddBtn])))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for Vmware splashscreen")
	}

	if err := uiauto.Combine("click on adding new connection and wait to next screen",
		k.AccelAction("Tab"),
		k.AccelAction("Enter"),
		c.detector.WaitUntilExists(uidetection.Word("Connect")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed clicking on add new connection")
	}

	testing.ContextLog(ctx, "Enter server url")
	if err := uiauto.Combine("enter Wmware server url, connect and wait for next screen",
		k.TypeAction(cfg.Server),
		k.AccelAction("Enter"),
		c.detector.WaitUntilExists(uidetection.Word("Login")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed adding server url")
	}

	testing.ContextLog(ctx, "Enter username and password")
	if err := uiauto.Combine("enter username and password and connect login",
		k.AccelAction("Tab"),
		k.TypeAction(cfg.Username),
		k.AccelAction("Tab"),
		k.TypeAction(cfg.Password),
		k.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering username or password")
	}

	return nil
}

// EnsureMainScreenVisible ensures that element visible on the screen
// indicates it is the main Vmware screen.
func (c *Connector) EnsureMainScreenVisible(ctx context.Context) error {
	if err := c.detector.WaitUntilExists(uidetection.Word("Horizon"))(ctx); err != nil {
		return errors.Wrap(err, "didn't see expected text after logging into Wmware")
	}
	return nil
}

// SearchAndOpenApplication opens given application using search provided in
// VMware. Call ResetSearch() to clean the search state.
func (c *Connector) SearchAndOpenApplication(ctx context.Context, k *input.KeyboardEventWriter, appName string) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Wmware - opening %s app", appName)
		return uiauto.Combine("open "+appName+" application in Wmware",
			k.TypeAction(appName),
			k.AccelAction("Shift+Tab"), // Cycle back to the applications.
			k.AccelAction("Shift+Tab"), // Select the first result.
			k.AccelAction("Enter"),     // Launch first app.
		)(ctx)
	}
}

// ResetSearch cleans search field.
func (c *Connector) ResetSearch(ctx context.Context, k *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Wmware - clean search")

	// Need to make sure test is on the main screen.
	if err := c.EnsureMainScreenVisible(ctx); err != nil {
		return errors.Wrap(err, "can't proceed with clearing search results")
	}

	// Make sure "Search" is not visible - that means it was used and we need to clear it.
	if _, err := c.detector.Location(ctx, uidetection.Word("Search")); err == nil {
		return errors.Wrap(err, "found 'Search' when clearing search results. Loos like there is nothing to clean")
	}
	if err := uiauto.Combine("clearing search results",
		k.AccelAction("Tab"),
		k.AccelAction("Tab"),       // Select search phrase,
		k.AccelAction("Backspace"), // Clear the selection.
		k.AccelAction("Esc"),       // Return to the view.
	)(ctx); err != nil {
		return errors.Wrap(err, "couldn't clear the search results")
	}
	return nil
}
