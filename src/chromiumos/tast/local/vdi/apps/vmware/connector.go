// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmware

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

// Connector structure used for performing operation on VMware app.
type Connector struct {
	dataPath func(string) string
	detector *uidetection.Context
	tconn    *chrome.TestConn
	keyboard *input.KeyboardEventWriter
	cfg      *apps.VDILoginConfig
}

// Init initializes state of the connector.
func (c *Connector) Init(s *testing.FixtState, tconn *chrome.TestConn, d *uidetection.Context, k *input.KeyboardEventWriter) {
	c.dataPath = s.DataPath
	c.detector = d
	c.tconn = tconn
	c.keyboard = k
}

// EnterServerURL waits for the screen and enters url to the VMWare setup.
func (c *Connector) EnterServerURL(ctx context.Context, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "VMWare: entering server url")

	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.CustomIcon(c.dataPath(VmwareData[SplashscreenAddBtn])))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for VMware splashscreen")
	}

	if err := uiauto.Combine("click on adding new connection and wait to next screen",
		c.detector.LeftClick(uidetection.CustomIcon(c.dataPath(VmwareData[SplashscreenAddBtn]))),
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Connect")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed clicking on add new connection")
	}

	testing.ContextLog(ctx, "VMware: entering server url")
	if err := uiauto.Combine("enter VMware server url, connect and wait for next screen",
		c.keyboard.TypeAction(cfg.Server),
		c.keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed adding server url")
	}

	return nil
}

// EnterCredentialsAndLogin waits for the screen and enters credentials.
func (c *Connector) EnterCredentialsAndLogin(ctx context.Context, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "VMWare: entering username and password and logging in")
	if err := uiauto.Combine("enter username and password and connect login",
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Login")),
		c.keyboard.TypeAction(cfg.Username),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.TypeAction(cfg.Password),
		c.keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering username or password")
	}

	return nil
}

// Login connects to the server and logs in using information provided in config.
func (c *Connector) Login(ctx context.Context, cfg *apps.VDILoginConfig) error {
	testing.ContextLog(ctx, "VMware: logging in")
	c.cfg = cfg // It is needed for reauth after re opening app.

	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.CustomIcon(c.dataPath(VmwareData[SplashscreenAddBtn])))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for VMware splashscreen")
	}

	if err := uiauto.Combine("click on adding new connection and wait to next screen",
		c.detector.LeftClick(uidetection.CustomIcon(c.dataPath(VmwareData[SplashscreenAddBtn]))),
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Connect")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed clicking on add new connection")
	}

	testing.ContextLog(ctx, "VMware: entering server url")
	if err := uiauto.Combine("enter VMware server url, connect and wait for next screen",
		c.keyboard.TypeAction(cfg.Server),
		c.keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed adding server url")
	}

	testing.ContextLog(ctx, "VMware: entering username and password")
	if err := uiauto.Combine("enter username and password and connect login",
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Login")),
		c.keyboard.TypeAction(cfg.Username),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.TypeAction(cfg.Password),
		c.keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering username or password")
	}

	return nil
}

// Logout logs out user. Can be called when VMware logged-in main screen is
// visible.
func (c *Connector) Logout(ctx context.Context) error {
	// In user session it is enough to close the window with Ctrl+W. However,
	// it has to be checked how it works on Kiosk. TODO: kamilszarek.
	testing.ContextLog(ctx, "VMware: logging out")
	if err := uiauto.Combine("enter password and login",
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Tab"), // Go to logout button.
		c.keyboard.AccelAction("Enter"),
		c.keyboard.AccelAction("Enter"), // Confirm logging out.
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to log out")
	}

	return nil
}

// LoginAfterRestart handles special case when application does not cache the
// session and requires login after app restarts.
func (c *Connector) LoginAfterRestart(ctx context.Context) error {
	testing.ContextLog(ctx, "VMware: selecting server")
	if err := uiauto.Combine("select server and hit enter",
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.CustomIcon(c.dataPath(VmwareData[SplashscreenAddBtn]))),
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Enter"),
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Login")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to server")
	}

	testing.ContextLog(ctx, "VMware: entering password and logging in")
	if err := uiauto.Combine("enter password and login",
		c.keyboard.TypeAction(c.cfg.Password),
		c.keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed entering username or password")
	}

	return nil
}

// WaitForMainScreenVisible waits for the specific element to be visible in
// the UI indicating VMware Horizon app is on its main screen.
func (c *Connector) WaitForMainScreenVisible(ctx context.Context) error {
	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Horizon"))(ctx); err != nil {
		return errors.Wrap(err, "didn't see expected text after logging into VMware")
	}
	return nil
}

// SearchAndOpenApplication opens given application using search provided in
// VMware, runs checkIfOpened function to ensure app opened. Before calling
// make sure main VMware screen is visible by calling
// WaitForMainScreenVisible(). Call ResetSearch() to clean the search state.
func (c *Connector) SearchAndOpenApplication(ctx context.Context, appName string, checkIfOpened func(context.Context) error) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "VMware: opening %s app", appName)
		return uiauto.Combine("open "+appName+" application in VMware",
			c.keyboard.TypeAction(appName),      // Focus is already on search field.
			c.keyboard.AccelAction("Shift+Tab"), // Cycle away from search field.
			c.keyboard.AccelAction("Shift+Tab"), // Cycle back to the applications.
			c.keyboard.AccelAction("Shift+Tab"), // Select the first star bookmarking.
			c.keyboard.AccelAction("Shift+Tab"), // Select the first result icon.
			c.keyboard.AccelAction("Enter"),     // Launch first app.
			checkIfOpened,
		)(ctx)
	}
}

// ResetSearch cleans search field. Call only when search was triggered by
// SearchAndOpenApplication().
func (c *Connector) ResetSearch(ctx context.Context) error {
	testing.ContextLog(ctx, "VMware: cleaning search")

	// Need to make sure test is on the main screen.
	if err := c.WaitForMainScreenVisible(ctx); err != nil {
		return errors.Wrap(err, "can't proceed with clearing search results")
	}

	// Make sure "Search" is not visible - that means it was used and we need to clear it.
	if _, err := c.detector.Location(ctx, uidetection.Word("Search")); err == nil {
		return errors.Wrap(err, "found 'Search' when clearing search results. Loos like there is nothing to clean")
	}
	// VMware, after executed search, opened and closed app does not keep
	// focus on the search field. Hence we enter it with 2xTab, clean it and
	// leave the focus.
	if err := uiauto.Combine("clearing search results",
		c.keyboard.AccelAction("Tab"),
		c.keyboard.AccelAction("Tab"),       // Select search phrase,
		c.keyboard.AccelAction("Backspace"), // Clear the selection.
	)(ctx); err != nil {
		return errors.Wrap(err, "couldn't clear the search results")
	}
	return nil
}

// ReplaceDetector replaces detector instance.
func (c *Connector) ReplaceDetector(d *uidetection.Context) {
	c.detector = d
}
