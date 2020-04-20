// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cui contains functions to interact with the ChromeOS parts of the crostini UI.
// This is primarily the settings and the installer.
package cui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
)

// Settings is a page object for the Crostini section of the settings app.
type Settings struct {
	tconn *chrome.TestConn
}

// Installer is a page object for the settings screen of the Crostini Installer.
type Installer struct {
	tconn *chrome.TestConn
}

// OpenSettings opens the settings app (if needed) and returns a settings page object.
//
// It also hides all notifications to ensure subsequent operations work correctly.
func OpenSettings(ctx context.Context, tconn *chrome.TestConn) (*Settings, error) {
	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to hide all notifications in OpenSettings()")
	}
	p := &Settings{tconn}
	err := p.ensureOpen(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error in OpenSettings()")
	}
	return p, err
}

// ensureOpen checks if the settings app is open, and opens it if it is not.
func (p *Settings) ensureOpen(ctx context.Context) error {
	shown, err := ash.AppShown(ctx, p.tconn, apps.Settings.ID)
	if err != nil {
		return err
	}
	if shown {
		return nil
	}
	if err := apps.Launch(ctx, p.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch settings app")
	}
	if err := ash.WaitForApp(ctx, p.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "Settings app did not appear in the shelf")
	}
	return nil
}

// OpenInstaller clicks the "Turn on" Linux button to open the Crostini installer.
//
// It also clicks next to skip the information screen.  The returned Installer
// page object can be used to adjust the settings and to complete the installation.
func (p *Settings) OpenInstaller(ctx context.Context) (*Installer, error) {
	if err := p.ensureOpen(ctx); err != nil {
		return nil, errors.Wrap(err, "error in OpenInstaller()")
	}
	return &Installer{p.tconn}, uig.Do(ctx, p.tconn,
		uig.Steps(
			uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Linux (Beta)"}, 10*time.Second).FocusAndWait(3*time.Second).LeftClick(),
			uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Next"}, 10*time.Second).LeftClick()).
			WithName("OpenInstaller()"))
}

// Install clicks the install button and waits for the Linux installation to complete.
func (p *Installer) Install(ctx context.Context) error {
	return uig.Do(ctx, p.tconn,
		uig.Steps(
			uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Install"}, 10*time.Second).LeftClick(),
			uig.WaitUntilDescendantGone(ui.FindParams{Role: ui.RoleTypeButton, Name: "Cancel"}, 2*time.Minute)).
			WithName("Install()"))
}
