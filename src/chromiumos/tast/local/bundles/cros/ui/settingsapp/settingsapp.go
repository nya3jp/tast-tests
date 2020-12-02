// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settingsapp supports controlling the Settings App on Chrome OS.
package settingsapp

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// Links of the Settings app.
const (
	Network   = "Network"
	Bluetooth = "Bluetooth"
	WiFi      = "Wi-Fi"
)

const uiTimeout = 15 * time.Second

var rootFindParams = ui.FindParams{
	Role:       ui.RoleTypeWindow,
	ClassName:  "BrowserFrame",
	Attributes: map[string]interface{}{"name": regexp.MustCompile("Settings")},
}

// SettingsApp represents an instance of the Settings App.
type SettingsApp struct {
	tconn *chrome.TestConn
	Root  *ui.Node
}

// Launch launches the Settings App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*SettingsApp, error) {
	// Launch the Settings App.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		return nil, err
	}

	// Get Settings App root node.
	app, err := ui.FindWithTimeout(ctx, tconn, rootFindParams, time.Minute)
	if err != nil {
		return nil, err
	}

	return &SettingsApp{tconn: tconn, Root: app}, nil
}

// Close closes the Settings App.
func (s *SettingsApp) Close(ctx context.Context) error {
	s.Root.Release(ctx)

	// Close the Settings App.
	if err := apps.Close(ctx, s.tconn, apps.Settings.ID); err != nil {
		return err
	}

	// Wait for window to close.
	return ui.WaitUntilGone(ctx, s.tconn, rootFindParams, time.Minute)
}

// NavigateTo navigates to the linked section.
func (s *SettingsApp) NavigateTo(ctx context.Context, link string) error {
	// Click the menu button if it exists
	menu, err := s.Root.Descendant(ctx, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Main menu",
	})
	if err == nil {
		defer menu.Release(ctx)
		menu.LeftClick(ctx)
		testing.Sleep(ctx, time.Second) // wait for the drawer shows up
	}

	params := ui.FindParams{
		Role: ui.RoleTypeLink,
		Name: link,
	}
	return cuj.WaitAndClickDescendant(ctx, s.Root, params, uiTimeout)
}

// OpenSubpage opens a subpage by clicking the arrow button of the named row.
func (s *SettingsApp) OpenSubpage(ctx context.Context, name string) error {
	params := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      name,
		ClassName: "subpage-arrow",
	}
	return cuj.WaitAndClickDescendant(ctx, s.Root, params, uiTimeout)
}

func (s *SettingsApp) toggleButton(ctx context.Context, name string) (*ui.Node, error) {
	return s.Root.Descendant(ctx, ui.FindParams{
		Role:       ui.RoleTypeToggleButton,
		Attributes: map[string]interface{}{"name": regexp.MustCompile(name)},
	})
}

// SwitchOn enables the named toggle button.
func (s *SettingsApp) SwitchOn(ctx context.Context, name string) error {
	btn, err := s.toggleButton(ctx, name)
	if err != nil {
		return err
	}
	defer btn.Release(ctx)

	if btn.Checked == ui.CheckedStateMixed {
		return errors.New("button is not toggleable")
	}
	if btn.Checked != ui.CheckedStateTrue {
		return btn.LeftClick(ctx)
	}
	return nil
}

// SwitchOff disables the named toggle button.
func (s *SettingsApp) SwitchOff(ctx context.Context, name string) error {
	btn, err := s.toggleButton(ctx, name)
	if err != nil {
		return err
	}
	defer btn.Release(ctx)

	if btn.Checked == ui.CheckedStateMixed {
		return errors.New("button is not toggleable")
	}
	if btn.Checked == ui.CheckedStateTrue {
		return btn.LeftClick(ctx)
	}
	return nil
}
