// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package citrix

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/testing"
)

const uiDetectionTimeout = 45 * time.Second

// Connector structure used for performing operation on Citrix app.
type Connector struct {
	dataPath func(string) string
	detector *uidetection.Context
	tconn    *chrome.TestConn
	keyboard *input.KeyboardEventWriter
}

// Init initializes state of the connector.
func (c *Connector) Init(s *testing.FixtState, tconn *chrome.TestConn, d *uidetection.Context, k *input.KeyboardEventWriter) {
	c.dataPath = s.DataPath
	c.detector = d
	c.tconn = tconn
	c.keyboard = k
}

// EnterServerURL enters url to the Citix setup.
func (c *Connector) EnterServerURL(ctx context.Context, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "Citrix: entering server url")

	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("https://URL"))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for Citrix splashscreen")
	}

	testing.ContextLog(ctx, "Citrix: entering server url")
	if err := uiauto.Combine("enter citrix server url, connect and wait for next screen",
		c.keyboard.AccelAction("Tab"), // Enter server test box.
		c.keyboard.TypeAction(cfg.Server),
		c.keyboard.AccelAction("Enter"), // Connect to the server.
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering server url")
	}

	return nil
}

// EnterCredentialsAndLogin waits for the screen and enters credentials.
func (c *Connector) EnterCredentialsAndLogin(ctx context.Context, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "Citrix: entering username and password and logging in")
	if err := uiauto.Combine("enter username and password and connect login",
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"User", "name"})),
		c.keyboard.TypeAction(cfg.Username),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.TypeAction(cfg.Password),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering username or password")
	}

	return nil
}

// Login connects to the server and logs in using information provided in config.
func (c *Connector) Login(ctx context.Context, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "Citrix: logging in")

	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("https://URL"))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for Citrix splashscreen")
	}

	testing.ContextLog(ctx, "Citrix: entering server url")
	if err := uiauto.Combine("enter citrix server url, connect and wait for next screen",
		c.keyboard.AccelAction("Tab"), // Enter server test box.
		c.keyboard.TypeAction(cfg.Server),
		c.keyboard.AccelAction("Enter"), // Connect to the server.
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering server url")
	}

	testing.ContextLog(ctx, "Citrix: entering username and password")
	if err := uiauto.Combine("enter username and password and connect login",
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"User", "name"})),
		c.keyboard.TypeAction(cfg.Username),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.TypeAction(cfg.Password),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering username or password")
	}

	return nil
}

// Logout logs out user. Can be called when Citrix logged-in main screen is
// visible.
func (c *Connector) Logout(ctx context.Context) error {
	testing.ContextLog(ctx, "Citrix: logging out")
	if err := uiauto.Combine("log out from the Citrix",
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"Citrix", "Workspace"})),
		c.detector.LeftClick(uidetection.TextBlock([]string{"Citrix", "Workspace"})), // By clicking, focus on the first UI element.
		c.detector.LeftClick(uidetection.Word("C").ExactMatch()),                     // Click on the user icon.
		c.detector.LeftClick(uidetection.TextBlock([]string{"Log", "Out"})),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to log out")
	}

	return nil
}

// LoginAfterRestart handles special case when application does not cache the
// session and requires login after app restarts.
func (c *Connector) LoginAfterRestart(ctx context.Context) error {
	// Do nothing since Citrix session is cached by the app and login is not
	// needed.
	testing.ContextLog(ctx, "Citrix: already logged in")
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
func (c *Connector) SearchAndOpenApplication(ctx context.Context, appName string, checkIfOpened func(context.Context) error) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Citrix: opening app %s", appName)
		return uiauto.Combine("open application "+appName+" in Citrix",
			// kamilszarek@: I tried using uidetector clicking on test block
			// "Search Workspace" and it wound click it but focus does not
			// go on the search field.
			c.keyboard.AccelAction("Tab"),
			c.keyboard.AccelAction("Tab"),
			c.keyboard.AccelAction("Tab"),   // Go to the search field.
			c.keyboard.AccelAction("Enter"), // Move focus to the search field.
			c.keyboard.TypeAction(appName),
			c.keyboard.AccelAction("Down"),  // Go to the first result.
			c.keyboard.AccelAction("Enter"), // Open the app.
			checkIfOpened,
		)(ctx)
	}
}

// ResetSearch cleans search field. Call only when search was triggered by
// SearchAndOpenApplication().
func (c *Connector) ResetSearch(ctx context.Context) error {
	testing.ContextLog(ctx, "Citrix: cleaning search")
	// Check that result was actually triggered.
	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.TextBlock([]string{"See", "more", "results"}))(ctx); err != nil {
		return errors.Wrap(err, "search results view is not visible")
	}
	// Citrix, after executed search and the app was opened and then closed
	// keeps the search result overlay on with focus on it.
	if err := uiauto.Combine("clearing search results",
		c.keyboard.AccelAction("Esc"), // Return to the Citrix view.
	)(ctx); err != nil {
		return errors.Wrap(err, "couldn't clear the search results")
	}
	return nil
}

// ReplaceDetector replaces detector instance.
func (c *Connector) ReplaceDetector(d *uidetection.Context) {
	c.detector = d
}
