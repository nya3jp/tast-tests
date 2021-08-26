// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanapp supports controlling the Scan App on Chrome OS.
package scanapp

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
)

// WindowFinder is the finder for the ScanApp window.
var WindowFinder *nodewith.Finder = nodewith.Name(apps.Scan.Name).ClassName("BrowserFrame").Role(role.Window)

var scanButtonFinder *nodewith.Finder = nodewith.Name("Scan").Role(role.Button)

var doneButtonFinder *nodewith.Finder = nodewith.Name("Done").Role(role.Button)

// ScanApp represents an instance of the Scan App.
type ScanApp struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
}

// DropdownName defines the name of a dropdown.
type DropdownName string

// The names for each of the Scan app's dropdowns.
const (
	DropdownNameScanner    DropdownName = "Scanner"
	DropdownNameSource     DropdownName = "Source"
	DropdownNameScanTo     DropdownName = "Scan to"
	DropdownNameFileType   DropdownName = "File type"
	DropdownNameColorMode  DropdownName = "Color"
	DropdownNamePageSize   DropdownName = "Page size"
	DropdownNameResolution DropdownName = "Resolution"
)

// Source defines a source option.
type Source string

// The available source options.
const (
	SourceFlatbed     Source = "Flatbed"
	SourceADFOneSided Source = "Document Feeder (One-sided)"
	SourceADFTwoSided Source = "Document Feeder (Two-sided)"
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
	PageSizeA3            PageSize = "A3"
	PageSizeA4            PageSize = "A4"
	PageSizeB4            PageSize = "B4"
	PageSizeLegal         PageSize = "Legal"
	PageSizeLetter        PageSize = "Letter"
	PageSizeTabloid       PageSize = "Tabloid"
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

// Launch launches the Scan App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*ScanApp, error) {
	// Launch the Scan App.
	if err := apps.Launch(ctx, tconn, apps.Scan.ID); err != nil {
		return nil, err
	}

	// Create a uiauto.Context with default timeout.
	ui := uiauto.New(tconn)

	s := ScanApp{tconn: tconn, ui: ui}

	// Wait until the scan button is enabled to verify the app is loaded.
	if err := s.WithTimeout(time.Minute).WaitUntilExists(scanButtonFinder)(ctx); err != nil {
		return nil, err
	}

	return &s, nil
}

// Close closes the Scan App.
// This is automatically done when Chrome resets and is not necessary to call.
func (s *ScanApp) Close(ctx context.Context) error {
	// Close the Scan App.
	if err := apps.Close(ctx, s.tconn, apps.Scan.ID); err != nil {
		return err
	}

	// Wait for window to close.
	return s.WithTimeout(time.Minute).WaitUntilGone(WindowFinder)(ctx)
}

// ClickMoreSettings returns a function that clicks the More settings button to
// expand or collapse the content.
func (s *ScanApp) ClickMoreSettings() uiauto.Action {
	return s.LeftClick(nodewith.Name("More settings").Role(role.Button))
}

// SetScanSettings returns a function that interacts with the Scan app to set
// the scan settings.
func (s *ScanApp) SetScanSettings(settings ScanSettings) uiauto.Action {
	var steps []uiauto.Action
	for _, dropdown := range []struct {
		name   DropdownName
		option string
	}{
		{DropdownNameScanner, settings.Scanner},
		{DropdownNameSource, string(settings.Source)},
		{DropdownNameFileType, string(settings.FileType)},
		{DropdownNameColorMode, string(settings.ColorMode)},
		{DropdownNamePageSize, string(settings.PageSize)},
		{DropdownNameResolution, string(settings.Resolution)},
	} {
		dropdownFinder := nodewith.Name(string(dropdown.name)).ClassName("md-select")
		dropdownOptionFinder := nodewith.Name(dropdown.option).Role(role.ListBoxOption)
		steps = append(steps, s.LeftClick(dropdownFinder), s.LeftClick(dropdownOptionFinder))
	}

	return uiauto.Combine("SetScanSettings", steps...)
}

// Scan returns a function that performs a scan by clicking the scan button.
func (s *ScanApp) Scan() uiauto.Action {
	return uiauto.Combine("scan",
		s.LeftClick(scanButtonFinder),
		// Wait until the done button is displayed to verify the scan completed
		// successfully.
		s.WaitUntilExists(doneButtonFinder),
	)
}

// ClickMyFilesLink returns a function that opens My files in the Files app by
// clicking the My files folder link.
func (s *ScanApp) ClickMyFilesLink() uiauto.Action {
	return s.LeftClick(nodewith.Name("My files").Role(role.StaticText).State(state.Linked, true))
}

// ClickDone returns a function that clicks the done button to return to the
// first page of the app.
func (s *ScanApp) ClickDone() uiauto.Action {
	return s.LeftClick(doneButtonFinder)
}
