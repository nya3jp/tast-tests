// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanapp contains functions used to interact with the Scan SWA.
package scanapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

var defaultStablePollOpts = testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 15 * time.Second}

var scanAppRootNodeParams = ui.FindParams{
	Name: apps.Scan.Name,
	Role: ui.RoleTypeWindow,
}

var scanButtonParams = ui.FindParams{
	Name: "Scan",
	Role: ui.RoleTypeButton,
}

var doneButtonParams = ui.FindParams{
	Name: "Done",
	Role: ui.RoleTypeButton,
}

// ScanApp represents an instance of the Scan app.
type ScanApp struct {
	tconn          *chrome.TestConn
	Root           *ui.Node
	stablePollOpts *testing.PollOptions
}

func expandDropdown(ctx context.Context, tconn *chrome.TestConn, id string) error {
	params := ui.FindParams{ClassName: "md-select"}
	dropdowns, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		return errors.Wrap(err, "failed to find dropdowns")
	}
	defer dropdowns.Release(ctx)

	for _, dropdown := range dropdowns {
		if dropdown.HTMLAttributes["id"] == id {
			if err := dropdown.LeftClick(ctx); err != nil {
				return err
			}

			return nil
		}
	}

	return errors.New("failed to find dropdown")
}

func (s *ScanApp) waitForScanButton(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		scanButton, err := s.Root.DescendantWithTimeout(ctx, scanButtonParams, uiTimeout)
		if err != nil {
			return errors.Wrap(err, "failed to find scan button")
		}
		defer scanButton.Release(ctx)

		if scanButton.Restriction == ui.RestrictionDisabled {
			return errors.New("scan button is disabled")
		}

		return nil
	}, s.stablePollOpts); err != nil {
		return errors.Wrap(err, "failed to wait for scan button")
	}

	return nil
}

// Launch launches the Scan app and returns it. An error is returned if the app
// fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*ScanApp, error) {
	// Launch the Scan app.
	if err := apps.Launch(ctx, tconn, apps.Scan.ID); err != nil {
		return nil, err
	}

	// Get the Scan app root node.
	root, err := ui.FindWithTimeout(ctx, tconn, scanAppRootNodeParams, time.Minute)
	if err != nil {
		return nil, err
	}

	app := ScanApp{tconn: tconn, Root: root, stablePollOpts: &defaultStablePollOpts}

	// Wait until the scan button is enabled to verify the app is loaded.
	if err := app.waitForScanButton(ctx); err != nil {
		return nil, err
	}

	return &app, nil
}

// Release releases the root node held by the Scan app.
func (s *ScanApp) Release(ctx context.Context) {
	s.Root.Release(ctx)
}

// SelectOption selects the option defined by name in the dropdown defined by
// id.
func (s *ScanApp) SelectOption(ctx context.Context, id, name string) error {
	if err := expandDropdown(ctx, s.tconn, id); err != nil {
		return errors.Wrap(err, "failed to click dropdown")
	}

	params := ui.FindParams{
		Name: name,
		Role: ui.RoleTypeListBoxOption,
	}
	option, err := s.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find option")
	}
	defer option.Release(ctx)

	if err := option.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click option")
	}

	// Wait for the app to start processing the selected option.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}

	return nil
}

// Scan performs a scan by clicking the scan button once it's enabled.
func (s *ScanApp) Scan(ctx context.Context) error {
	if err := s.waitForScanButton(ctx); err != nil {
		return err
	}

	scanButton, err := s.Root.DescendantWithTimeout(ctx, scanButtonParams, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find scan button")
	}
	defer scanButton.Release(ctx)

	if err := scanButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click scan button")
	}

	// Wait until the done button is displayed to verify the scan completed
	// successfully.
	if err := s.Root.WaitUntilDescendantExists(ctx, doneButtonParams, 30*time.Second); err != nil {
		return errors.New("scan failed to complete")
	}

	return nil
}

// WaitForApp waits for the Scan app to be shown and rendered.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn) error {
	appRootNode, err := ui.FindWithTimeout(ctx, tconn, scanAppRootNodeParams, time.Minute)
	if err != nil {
		return errors.Wrap(err, "failed to find Scan app")
	}
	defer appRootNode.Release(ctx)

	// Find the scan button to verify the app is rendering.
	if err := appRootNode.WaitUntilDescendantExists(ctx, scanButtonParams, uiTimeout); err != nil {
		return errors.Wrap(err, "failed to render Scan app")
	}

	return nil
}
