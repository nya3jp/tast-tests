// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package citrix

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/testing"
)

const uiDetectionTimeout = 45 * time.Second

// Connector structure used for performing operation on Citrix app.
type Connector struct {
	ApplicationID string
	dataPath      func(string) string
	detector      *uidetection.Context
}

// Init initializes state of the connector.
func (c *Connector) Init(s *testing.FixtState, d *uidetection.Context) {
	c.dataPath = s.DataPath
	c.detector = d
}

// Login connects to the server and logs in using information provided in config.
func (c *Connector) Login(ctx context.Context, k *input.KeyboardEventWriter, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "Citrix: logging in")

	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("https://URL"))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for Citrix splashscreen")
	}

	testing.ContextLog(ctx, "Citrix: entering server url")
	if err := uiauto.Combine("enter citrix server url, connect and wait for next screen",
		k.AccelAction("Tab"), // Enter server test box.
		k.TypeAction(cfg.Server),
		k.AccelAction("Enter"), // Connect to the server.
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"User", "name"})),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering server url")
	}

	testing.ContextLog(ctx, "Citrix: entering username and password")
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
	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"Search", "Workspace"}))(ctx); err != nil {
		return errors.Wrap(err, "didn't see expected text block after logging into Citrix")
	}

	return nil
}

// SearchAndOpenApplication opens given application using search provided in
// Citrix, runs checkIfOpened function to ensure app opened. Before calling
// make sure main Citrix screen is visible by calling
// WaitForMainScreenVisible(). Call ResetSearch() to clean the search state.
func (c *Connector) SearchAndOpenApplication(ctx context.Context, k *input.KeyboardEventWriter, appName string, checkIfOpened func(context.Context) error) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Citrix: opening app %s", appName)
		return uiauto.Combine("open application "+appName+" in Citrix",
			// kamilszarek@: I tried using uidetector clicking on test block
			// "Search Workspace" and it wound click it but focus does not
			// go on the search field.
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
func (c *Connector) ResetSearch(ctx context.Context, k *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Citrix: cleaning search")
	// Check that result was actually triggered.
	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"See", "more", "results"}))(ctx); err != nil {
		return errors.Wrap(err, "search results view is not visible")
	}
	// Citrix, after executed search and the app was opened and then closed
	// keeps the search result overlay on with focus on it.
	if err := uiauto.Combine("clearing search results",
		k.AccelAction("Ctrl+a"),    // Select search phrase,
		k.AccelAction("Backspace"), // Clear the selection.
		k.AccelAction("Esc"),       // Return to the Citrix view.
	)(ctx); err != nil {
		return errors.Wrap(err, "couldn't clear the search results")
	}
	return nil
}
