// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanapp supports controlling the Scan App on Chrome OS.
package scanapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
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
	Resolution200DPI  Resolution = "200 dpi"
	Resolution300DPI  Resolution = "300 dpi"
	Resolution600DPI  Resolution = "600 dpi"
	Resolution1200DPI Resolution = "1200 dpi"
)

// ToInt returns the integer representation of `r`.
func (r Resolution) ToInt() (int, error) {
	return strconv.Atoi(strings.TrimSuffix(string(r), " dpi"))
}

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
	return launchHelper(ctx, tconn, uiauto.New(tconn))
}

// LaunchWithPollOpts is like Launch, above, but allows the user to specify the
// PollOptions for the uiauto connection.
func LaunchWithPollOpts(ctx context.Context, opts testing.PollOptions, tconn *chrome.TestConn) (*ScanApp, error) {
	return launchHelper(ctx, tconn, uiauto.New(tconn).WithPollOpts(opts))
}

// launchHelper is a helper function for Launch and LaunchWithPollOpts.
func launchHelper(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) (*ScanApp, error) {
	// Launch the Scan App.
	if err := apps.Launch(ctx, tconn, apps.Scan.ID); err != nil {
		return nil, err
	}

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

// selectScanSetting is a helper function for the various SelectXXX functions
// which follow.
func (s *ScanApp) selectScanSetting(name DropdownName, value string) uiauto.Action {
	dropdownFinder := nodewith.Name(string(name)).ClassName("md-select")
	dropdownOptionFinder := nodewith.Name(value).Role(role.ListBoxOption)
	steps := []uiauto.Action{s.LeftClick(dropdownFinder), s.LeftClick(dropdownOptionFinder)}

	return uiauto.Combine(fmt.Sprintf("Select%s", string(name)), steps...)
}

// SelectScanner returns a function that interacts with the Scan app to select
// `scanner` from the list of detected scanners.
func (s *ScanApp) SelectScanner(scanner string) uiauto.Action {
	return s.selectScanSetting(DropdownNameScanner, scanner)
}

// SelectSource returns a function that interacts with the Scan app to select
// `source` from the list of supported sources.
func (s *ScanApp) SelectSource(source Source) uiauto.Action {
	return s.selectScanSetting(DropdownNameSource, string(source))
}

// SelectPageSize returns a function that interacts with the Scan app to select
// `pageSize` from the list of supported page sizes.
func (s *ScanApp) SelectPageSize(pageSize PageSize) uiauto.Action {
	return s.selectScanSetting(DropdownNamePageSize, string(pageSize))
}

// SelectColorMode returns a function that interacts with the Scan app to select
// `colorMode` from the list of supported color modes.
func (s *ScanApp) SelectColorMode(colorMode ColorMode) uiauto.Action {
	return s.selectScanSetting(DropdownNameColorMode, string(colorMode))
}

// SelectResolution returns a function that interacts with the Scan app to
// select `resolution` from the list of supported resolutions.
func (s *ScanApp) SelectResolution(resolution Resolution) uiauto.Action {
	return s.selectScanSetting(DropdownNameResolution, string(resolution))
}

// SelectFileType returns a function that interacts with the Scan app to
// select `fileType` from the list of supported file types.
func (s *ScanApp) SelectFileType(fileType FileType) uiauto.Action {
	return s.selectScanSetting(DropdownNameFileType, string(fileType))
}

// SetScanSettings returns a function that interacts with the Scan app to set
// the scan settings.
func (s *ScanApp) SetScanSettings(settings ScanSettings) uiauto.Action {
	steps := []uiauto.Action{
		s.SelectScanner(settings.Scanner),
		s.SelectSource(settings.Source),
		s.SelectFileType(settings.FileType),
		s.SelectColorMode(settings.ColorMode),
		s.SelectPageSize(settings.PageSize),
		s.SelectResolution(settings.Resolution),
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

// ClickMultiPageScanCheckbox returns a function that clicks the multi-page scan
// checkbox.
func (s *ScanApp) ClickMultiPageScanCheckbox() uiauto.Action {
	return s.LeftClick(nodewith.Role(role.CheckBox))
}

// MultiPageScan returns a function that performs a multi-page scan by clicking
// the scan button.
func (s *ScanApp) MultiPageScan(PageNumber int) uiauto.Action {
	return uiauto.Combine("multi-page scan",
		s.LeftClick(nodewith.Name("Scan page "+fmt.Sprintf("%d", PageNumber)).Role(role.Button)),

		// Wait until the 'Scan next page' button is displayed to verify the scan
		// completed successfully.
		s.WaitUntilExists(nodewith.Name("Scan page "+fmt.Sprintf("%d", PageNumber+1)).Role(role.Button)),
	)
}

// ClickSave returns a function that clicks the Save button to end a multi-page
// scan session.
func (s *ScanApp) ClickSave() uiauto.Action {
	return s.LeftClick(nodewith.Name("End & save").Role(role.Button))
}

// RemovePage returns a function that moves the mouse over the scan preview
// section and removes the current page in view.
func (s *ScanApp) RemovePage() uiauto.Action {
	return uiauto.Combine("remove page from multi-page scan",
		// Move the mouse to hover over the scan preview so the toolbar shows.
		s.ui.MouseMoveTo(nodewith.NameContaining("Scanning completed"), 0),
		// Click the Remove button on the toolbar to open the dialog.
		s.LeftClick(nodewith.Name("Remove").Role(role.Button)),
		// Click the dialog's Remove button to confirm the dialog and remove the page.
		s.LeftClick(nodewith.Name("Remove").Role(role.Button)),
	)
}

// RescanPage returns a function that moves the mouse over the scan preview
// section and rescans the current page in view.
func (s *ScanApp) RescanPage() uiauto.Action {
	return uiauto.Combine("rescan page in multi-page scan",
		// Move the mouse to hover over the scan preview so the toolbar shows.
		s.ui.MouseMoveTo(nodewith.NameContaining("Scanning completed"), 0),
		// Click the Rescan button on the toolbar to open the dialog.
		s.LeftClick(nodewith.Name("Rescan").Role(role.Button)),
		// Click the dialog's Rescan button to confirm the dialog and rescan the page.
		s.LeftClick(nodewith.Name("Rescan").Role(role.Button)),
	)
}
