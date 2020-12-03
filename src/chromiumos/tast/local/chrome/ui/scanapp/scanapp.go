// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanapp provides support for controlling and interacting with the
// Chrome OS Scan app.
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

var defaultStablePollOpts = testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: uiTimeout}

var appRootParams = ui.FindParams{
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

// DropdownID defines the HTML id attribute of a dropdown.
type DropdownID string

// The id attributes for each of the Scan app's dropdowns.
const (
	DropdownIDScanner    DropdownID = "scannerSelect"
	DropdownIDSource     DropdownID = "sourceSelect"
	DropdownIDScanTo     DropdownID = "scanToSelect"
	DropdownIDFileType   DropdownID = "fileTypeSelect"
	DropdownIDColorMode  DropdownID = "colorModeSelect"
	DropdownIDPageSize   DropdownID = "pageSizeSelect"
	DropdownIDResolution DropdownID = "resolutionSelect"
)

// Source defines a source option.
type Source string

// The available source options.
const (
	SourceFlatbed     Source = "Flatbed"
	SourceADFOneSided Source = "Document Feeder (One Sided)"
	SourceADFTwoSided Source = "Document Feeder (Two Sided)"
)

// FileType defines a file type option.
type FileType string

// The available file type options.
const (
	FileTypeJPG FileType = "JPG"
	FileTypePNG FileType = "PNG"
	FileTypePDF FileType = "PDF"
)

// ColorMode defines a color mode option.
type ColorMode string

// The available color mode options.
const (
	ColorModeBlackAndWhite ColorMode = "Black and white"
	ColorModeColor         ColorMode = "Color"
	ColorModeGrayscale     ColorMode = "Grayscale"
)

// PageSize defines a page size option.
type PageSize string

// The available page size options.
const (
	PageSizeA4            PageSize = "A4"
	PageSizeLetter        PageSize = "Letter"
	PageSizeFitToScanArea PageSize = "Fit to scan area"
)

// Resolution defines a resolution option.
type Resolution string

// The available resolution options.
const (
	Resolution75DPI   Resolution = "75 dpi"
	Resolution150DPI  Resolution = "150 dpi"
	Resolution300DPI  Resolution = "300 dpi"
	Resolution600DPI  Resolution = "600 dpi"
	Resolution1200DPI Resolution = "1200 dpi"
)

// ScanSettings defines the settings to use to perform a scan.
type ScanSettings struct {
	Scanner    string
	Source     Source
	FileType   FileType
	ColorMode  ColorMode
	PageSize   PageSize
	Resolution Resolution
}

// clickDropdown finds and clicks the dropdown identified by id.
func (s *ScanApp) clickDropdown(ctx context.Context, id DropdownID) error {
	params := ui.FindParams{ClassName: "md-select"}
	dropdowns, err := s.Root.Descendants(ctx, params)
	if err != nil {
		return errors.Wrap(err, "failed to find dropdowns")
	}
	defer dropdowns.Release(ctx)

	for _, dropdown := range dropdowns {
		if dropdown.HTMLAttributes["id"] == string(id) {
			if err := dropdown.LeftClick(ctx); err != nil {
				return err
			}

			return nil
		}
	}

	return errors.New("failed to find dropdown")
}

// selectDropdownOption selects the option identified by optionName in the
// dropdown identified by id.
func (s *ScanApp) selectDropdownOption(ctx context.Context, id DropdownID, optionName string) error {
	if err := s.clickDropdown(ctx, id); err != nil {
		return errors.Wrap(err, "failed to expand dropdown")
	}

	params := ui.FindParams{
		Name: optionName,
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

	// Give the app time to start processing the selected option.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}

	return nil
}

// waitForScanButtonEnabled waits until the scan button is enabled.
func (s *ScanApp) waitForScanButtonEnabled(ctx context.Context) error {
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
	appRoot, err := ui.FindWithTimeout(ctx, tconn, appRootParams, time.Minute)
	if err != nil {
		return nil, err
	}

	app := ScanApp{tconn: tconn, Root: appRoot, stablePollOpts: &defaultStablePollOpts}

	// Wait until the scan button is enabled to verify the app is loaded.
	if err := app.waitForScanButtonEnabled(ctx); err != nil {
		return nil, err
	}

	return &app, nil
}

// Release releases the root node held by the Scan app.
func (s *ScanApp) Release(ctx context.Context) {
	s.Root.Release(ctx)
}

// ClickMoreSettings clicks the More settings button to expand or collapse the
// content.
func (s *ScanApp) ClickMoreSettings(ctx context.Context) error {
	params := ui.FindParams{
		Name: "More settings",
		Role: ui.RoleTypeButton,
	}
	button, err := s.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find More settings button")
	}
	defer button.Release(ctx)

	if err := button.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click More settings button")
	}

	return nil
}

// SetScanSettings interacts with the Scan app to set the scan settings.
func (s *ScanApp) SetScanSettings(ctx context.Context, settings ScanSettings) error {
	for _, dropdown := range []struct {
		id     DropdownID
		option string
	}{
		{DropdownIDScanner, settings.Scanner},
		{DropdownIDSource, string(settings.Source)},
		{DropdownIDFileType, string(settings.FileType)},
		{DropdownIDColorMode, string(settings.ColorMode)},
		{DropdownIDPageSize, string(settings.PageSize)},
		{DropdownIDResolution, string(settings.Resolution)},
	} {
		if err := s.selectDropdownOption(ctx, dropdown.id, dropdown.option); err != nil {
			return errors.Wrapf(err, "failed to select %v from %v: %v", dropdown.option, dropdown.id, err)
		}
	}

	return nil
}

// Scan performs a scan by clicking the scan button once it's enabled.
func (s *ScanApp) Scan(ctx context.Context) error {
	if err := s.waitForScanButtonEnabled(ctx); err != nil {
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

// ClickDone clicks the done button to return to the first page of the app.
func (s *ScanApp) ClickDone(ctx context.Context) error {
	doneButton, err := s.Root.DescendantWithTimeout(ctx, doneButtonParams, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find done button")
	}
	defer doneButton.Release(ctx)

	if err := doneButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click done button")
	}

	return nil
}

// WaitForApp waits for the Scan app to be shown and rendered. Launch can be
// used instead if the goal is to launch the app and obtain a pointer to it.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn) error {
	appRoot, err := ui.FindWithTimeout(ctx, tconn, appRootParams, time.Minute)
	if err != nil {
		return errors.Wrap(err, "failed to find Scan app")
	}
	defer appRoot.Release(ctx)

	// Find the scan button to verify the app is rendering.
	if err := appRoot.WaitUntilDescendantExists(ctx, scanButtonParams, uiTimeout); err != nil {
		return errors.Wrap(err, "failed to render Scan app")
	}

	return nil
}
