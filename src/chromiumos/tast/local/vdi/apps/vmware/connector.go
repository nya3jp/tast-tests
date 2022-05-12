// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmware

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

// Connector structure used for performing operation on Vmware app.
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
	testing.ContextLog(ctx, "Vmware: logging in")

	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.CustomIcon(c.dataPath(VmwareData[SplashscreenAddBtn])))(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for Vmware splashscreen")
	}

	if err := uiauto.Combine("click on adding new connection and wait to next screen",
		k.AccelAction("Tab"),
		k.AccelAction("Enter"),
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Connect")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed clicking on add new connection")
	}

	testing.ContextLog(ctx, "Vmware: entering server url")
	if err := uiauto.Combine("enter Wmware server url, connect and wait for next screen",
		k.TypeAction(cfg.Server),
		k.AccelAction("Enter"),
		c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Login")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed adding server url")
	}

	testing.ContextLog(ctx, "Vmware: entering username and password")
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

// WaitForMainScreenVisible waits for the specific element to be visible in
// the UI indicating Vmware Horizon app is on its main screen.
func (c *Connector) WaitForMainScreenVisible(ctx context.Context) error {
	if err := c.detector.WithTimeout(uiDetectionTimeout).WaitUntilExists(uidetection.Word("Horizon"))(ctx); err != nil {
		return errors.Wrap(err, "didn't see expected text after logging into Wmware")
	}
	return nil
}

// SearchAndOpenApplication opens given application using search provided in
// VMware, runs checkIfOpened function to ensure app opened. Before calling
// make sure main Vmware screen is visible by calling
// WaitForMainScreenVisible(). Call ResetSearch() to clean the search state.
func (c *Connector) SearchAndOpenApplication(ctx context.Context, k *input.KeyboardEventWriter, appName string, checkIfOpened func(context.Context) error) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Vmware: opening %s app", appName)
		return uiauto.Combine("open "+appName+" application in Wmware",
			k.TypeAction(appName),      // Focus is already on search field.
			k.AccelAction("Shift+Tab"), // Cycle back to the applications.
			k.AccelAction("Shift+Tab"), // Select the first star bookmarking.
			k.AccelAction("Shift+Tab"), // Select the first result icon.
			k.AccelAction("Enter"),     // Launch first app.
			checkIfOpened,
		)(ctx)
	}
}

// ResetSearch cleans search field. Call only when search was triggered by
// SearchAndOpenApplication().
func (c *Connector) ResetSearch(ctx context.Context, k *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Vmware: cleaning search")

	// Need to make sure test is on the main screen.
	if err := c.WaitForMainScreenVisible(ctx); err != nil {
		return errors.Wrap(err, "can't proceed with clearing search results")
	}

	// Make sure "Search" is not visible - that means it was used and we need to clear it.
	if _, err := c.detector.Location(ctx, uidetection.Word("Search")); err == nil {
		return errors.Wrap(err, "found 'Search' when clearing search results. Loos like there is nothing to clean")
	}
	// Vmware, after executed search, opened and closed app does not keep
	// focus on the search field. Hence we enter it with 2xTab, clean it and
	// leave the focus.
	if err := uiauto.Combine("clearing search results",
		k.AccelAction("Tab"),
		k.AccelAction("Tab"),       // Select search phrase,
		k.AccelAction("Backspace"), // Clear the selection.
	)(ctx); err != nil {
		return errors.Wrap(err, "couldn't clear the search results")
	}
	return nil
}
