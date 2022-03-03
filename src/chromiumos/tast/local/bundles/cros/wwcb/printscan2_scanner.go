// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// "Pre-Condition:
// - Printers should be MFC and connected to the same network or via USB cable
// - Tested devices -  Chromebook(ARM), Slate(tablet)(x86), Chromebox(x86)
// - Document is in scanner(flatbed or doc feeder) and ready to scan
// - [If not previously set] Go to Setting apps > Printers section, then select the printer and Add to Printers list

// Procedure:

// 1. Connect a scanner device through either
// -WiFi
// -USB

// 2. Open ‘Scan’ Utility app or go to Settings-> Print and Scan-> Scan.
// ----via Launcher - start typing Scan
// ----via ‘Scan’ in Settings

// 3. Observe the available selection options and choose:
// ----Scanning device from ‘Scanner’ (Default: Saved/Available Printers “A-Z”)
// ----Document feeder or Flatbed from ‘Source’(Default: Flatbed)
// *If one of these is missing, confirm the scanner device does not support it
// ----My Files or ‘Select folder in Files app…’ from ‘Scan to’,
//   *By clicking “Select folder in Files app…”, the Files app will open for selection (Downloads, Play files: Movies, Music, Pictures; Google Drive: My Drive; or New Folder)
// ----PDF, PNG, JPG from ‘File type’. (Default: PNG)

// 4. Observe the available selections for ‘More settings’ section and choose:
// ----Color or Grayscale from ‘Color mode’
// *If one of these is missing, confirm the scanner device does not support it
// ----Letter, A4 and Fit to scan Area
// ----75 dpi, 100 dpi, 200 dpi, 300 dpi, 600 dpi from ‘Resolution’. (Default: 300 dpi)

// 5. Start Scan

// 6. Observe
// ----""Scanning page x “status will show
// ----Scan in Progress (Stop and Close or Cancel)
// ----Scan completed should show a preview of the file
// ----File should be in the location that the ‘Scan to’ attribute is set, in the intended format chosen from the File type attribute.

// 5. Repeat the above steps with different combinations of 1. and 2. in combination with attributes in 4. And 5. Like these but not limited to:
// ----WiFi / Launcher / Flatbed, My Files / PDF / Color, Letter, 300dpi
// ----USB / Launcher / Flatbed, My Files / PNG / Grayscale, Letter, 100dpi
// ----WiFi / Settings / DocFeeder, Downloads / JPG / Grayscale, A4, 600dpi
// ----USB / Settings / DocFeeder, Downloads / PDF / Color, A4, 75dpi"

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// reference scanning.go
const (
	resolution100DPI scanapp.Resolution = "100 dpi"
	resolution200DPI scanapp.Resolution = "200 dpi"
)

type testingStruct struct {
	through    through
	via        via
	Source     scanapp.Source
	scanto     scanto
	ColorMode  scanapp.ColorMode
	PageSize   scanapp.PageSize
	FileType   scanapp.FileType
	Resolution scanapp.Resolution
}

type through string

const (
	throughUsb  through = "USB"
	throughWifi through = "WIFI"
)

type via string

const (
	viaLauncher via = "Launcher"
	viaSettings via = "Settings"
)

type scanto string

const (
	scantoMyFiles      scanto = "My files"
	scantoDownloads    scanto = "Downloads"
	scantoSelectFolder scanto = "Select folder in Files app…"
)

// Wi-Fi	Launcher	Flatbed,	My Files	Color	A4	PDF	75 dpi
// Wi-Fi	Launcher	Flatbed,	My Files	Grayscale	Letter	PNG	300 dpi
// Wi-Fi	Launcher	Document(One-sided), Feeder	Downloads	Color	Fit to scan area	JPG	150 dpi
// Wi-Fi	Launcher	Document(One-sided), Feeder	Downloads	Grayscale	A4	PDF	200 dpi
// Wi-Fi	Settings	Flatbed,	My Files	Color	Letter	PNG	100 dpi
// Wi-Fi	Settings	Flatbed,	My Files	Grayscale	Fit to scan area	JPG	600 dpi
// Wi-Fi	Settings	Document Feeder(One-sided),	Downloads	Color	A4	PDF	75 dpi
// Wi-Fi	Settings	Document Feeder(One-sided),	 Downloads	Grayscale	Letter	PNG	100 dpi
// USB	Launcher	Flatbed,	My Files	Color	Fit to scan area	JPG	150 dpi
// USB	Launcher	Flatbed,	My Files	Grayscale	A4	PDF	200 dpi
// USB	Launcher	Document Feeder(One-sided),	 Downloads	Color	Letter	PNG	300 dpi
// USB	Launcher	Document Feeder(One-sided),	 Downloads	Grayscale	Fit to scan area	JPG	600 dpi
// USB	Settings	Flatbed,	My Files	Color	A4	PDF	75 dpi
// USB	Settings	Flatbed,	My Files	Grayscale	Letter	PNG	100 dpi
// USB	Settings	Document Feeder(One-sided),	 Downloads	Color	Fit to scan area	JPG	150 dpi
// USB	Settings	Document Feeder(One-sided),	 Downloads	Grayscale	A4	PDF	200 dpi
var testingSettings = []testingStruct{
	{
		through:    throughWifi,
		via:        viaLauncher,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: scanapp.Resolution75DPI,
	},
	{
		through:    throughWifi,
		via:        viaLauncher,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: scanapp.Resolution300DPI,
	},
	{
		through:    throughWifi,
		via:        viaLauncher,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution150DPI,
	},
	{
		through:    throughWifi,
		via:        viaLauncher,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: resolution200DPI,
	},
	{
		through:    throughWifi,
		via:        viaSettings,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: resolution100DPI,
	},
	{
		through:    throughWifi,
		via:        viaSettings,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution600DPI,
	},
	{
		through:    throughWifi,
		via:        viaSettings,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: scanapp.Resolution75DPI,
	},
	{
		through:    throughWifi,
		via:        viaSettings,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: resolution100DPI,
	},
	{
		through:    throughUsb,
		via:        viaLauncher,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution150DPI,
	},
	{
		through:    throughUsb,
		via:        viaLauncher,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: resolution200DPI,
	},
	{
		through:    throughUsb,
		via:        viaLauncher,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: scanapp.Resolution300DPI,
	},
	{
		through:    throughUsb,
		via:        viaLauncher,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution600DPI,
	},
	{
		through:    throughUsb,
		via:        viaSettings,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: scanapp.Resolution75DPI,
	},
	{
		through:    throughUsb,
		via:        viaSettings,
		Source:     scanapp.SourceFlatbed,
		scanto:     scantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: resolution100DPI,
	},
	{
		through:    throughUsb,
		via:        viaSettings,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution150DPI,
	},
	{
		through:    throughUsb,
		via:        viaSettings,
		Source:     scanapp.SourceADFOneSided,
		scanto:     scantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: resolution200DPI,
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Printscan2Scanner,
		Desc:         "Connect to a scanning device and execute scanning job",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      time.Hour, // need to human be there, so set timeout longer
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"FixtureWebUrl"},
	})
}

func Printscan2Scanner(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// 1. Connect a scanner device through either
	// -WiFi
	// -USB
	if err := printscan2ScannerStep1(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step1: ")
	}

	usb, wifi, err := getScannerFromUI(ctx, s, tconn)
	if err != nil {
		s.Fatal("Failed to get scanner from ui: ", err)
	}

	// 2. Open ‘Scan’ Utility app or go to Settings-> Print and Scan-> Scan.
	// ----via Launcher - start typing Scan
	// ----via ‘Scan’ in Settings

	// 3. Observe the available selection options and choose:
	// ----Scanning device from ‘Scanner’ (Default: Saved/Available Printers “A-Z”)
	// ----Document feeder or Flatbed from ‘Source’(Default: Flatbed)
	// *If one of these is missing, confirm the scanner device does not support it
	// ----My Files or ‘Select folder in Files app…’ from ‘Scan to’,
	//   *By clicking “Select folder in Files app…”, the Files app will open for selection (Downloads, Play files: Movies, Music, Pictures; Google Drive: My Drive; or New Folder)
	// ----PDF, PNG, JPG from ‘File type’. (Default: PNG)
	// 4. Observe the available selections for ‘More settings’ section and choose:
	// ----Color or Grayscale from ‘Color mode’
	// *If one of these is missing, confirm the scanner device does not support it
	// ----Letter, A4 and Fit to scan Area
	// ----75 dpi, 100 dpi, 200 dpi, 300 dpi, 600 dpi from ‘Resolution’. (Default: 300 dpi)

	// 5. Start Scan
	// 6. ObserveprintFiles
	// ----""Scanning page x “status will show
	// ----Scan in Progress (Stop and Close or Cancel)
	// ----Scan completed should show a preview of the file
	// ----File should be in the location that the ‘Scan to’ attribute is set, in the intended format chosen from the File type attribute.
	// 5. Repeat the above steps with different combinations of 1. and 2. in combination with attributes in 4. And 5. Like these but not limited to:
	// ----WiFi / Launcher / Flatbed, My Files / PDF / Color, Letter, 300dpi
	// ----USB / Launcher / Flatbed, My Files / PNG / Grayscale, Letter, 100dpi
	// ----WiFi / Settings / DocFeeder, Downloads / JPG / Grayscale, A4, 600dpi
	// ----USB / Settings / DocFeeder, Downloads / PDF / Color, A4, 75dpi"
	if err := printscan2ScannerStep2To6(ctx, s, cr, tconn, usb, wifi); err != nil {
		s.Fatal("Failed to execute step2, 3, 4, 5, 6: ", err)
	}

}

func printscan2ScannerStep1(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 1 - Connect a scanner device through either")

	s.Log("Wifi - connect maunauly in advanced")

	s.Log("USB - plug in usb")
	if err := utils.ControlFixture(ctx, s, utils.USBPrinterType, utils.USBPrinterIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect usb")
	}

	// verfiy connected
	if _, err := ash.WaitForNotification(ctx, tconn, time.Minute, ash.WaitTitle("USB printer connected")); err != nil {
		s.Fatal("Failed to wait for notification: ", err)
	}

	return nil
}

func printscan2ScannerStep2To6(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, scannerUsb, scannerWifi string) error {

	s.Log("Step 2 - Open ‘Scan’ Utility app or go to Settings-> Print and Scan-> Scan")

	s.Log("Step 3, 4 - Observe the available selection options and choose")

	s.Log("Step 5, 6 - Repeat above steps with different combinations ")

	for _, ts := range testingSettings {

		var scanner string

		if ts.through == throughUsb {
			scanner = scannerUsb
		} else {
			scanner = scannerWifi
		}

		// launch scanapp as testingStruct via
		if err := launchScanapp(ctx, s, tconn, ts.via); err != nil {
			return err
		}

		// launch app
		app, err := scanapp.Launch(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to launch scan app")
		}

		// notice user to put file, when scan source is document one side
		if ts.Source == scanapp.SourceADFOneSided {
			// msg := fmt.Sprintf("Please put file into %s", scanapp.SourceADFOneSided)
			// if err := utils.WebNotification(ctx, s, msg); err != nil {
			// 	return errors.Wrap(err, "failed to show msg on server")
			// }
		}

		// set settings and perform scan
		settings := scanapp.ScanSettings{
			Scanner:    scanner,
			Source:     ts.Source,
			FileType:   ts.FileType,
			ColorMode:  ts.ColorMode,
			PageSize:   ts.PageSize,
			Resolution: ts.Resolution,
		}

		if err := app.ClickMoreSettings()(ctx); err != nil {
			return err
		}

		if err := app.SelectScanner(settings.Scanner)(ctx); err != nil {
			return err
		}

		// scan to
		if err := setScanto(ctx, s, tconn, ts.scanto); err != nil {
			return errors.Wrap(err, "failed to set scanto")
		}

		// Sleep to allow the supported sources to load and stabilize.
		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			return err
		}

		// set scan settings
		if err := uiauto.Combine("Set scan settings",
			app.SelectSource(settings.Source),
			app.SelectFileType(settings.FileType),
			app.SelectColorMode(settings.ColorMode),
			app.SelectPageSize(settings.PageSize),
			app.SelectResolution(settings.Resolution),
		)(ctx); err != nil {
			return err
		}

		startTime := time.Now()

		// perfor scan
		if err := app.WithTimeout(time.Minute).Scan()(ctx); err != nil {
			return errors.Wrap(err, "failed to perform scan")
		}

		app.Close(ctx)

		if err := verifyScanFile(ctx, s, startTime, &ts); err != nil {
			return err
		}

	}

	return nil

}

// launchScanapp launch scanapp via launcher or settings
func launchScanapp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, whichVia via) error {

	if whichVia == viaSettings {
		// via settings
		if err := launchScanappViaSettings(ctx, s, tconn); err != nil {
			return errors.Wrap(err, "failed to launch scanpp via settings")
		}
	} else {
		// via launcher
		if err := launchScanappViaLauncher(ctx, s, tconn); err != nil {
			return errors.Wrap(err, "failed to launch scanpp via launcher")
		}
	}

	// delay for window stablized
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return err
	}

	return nil
}

// launchScanappViaSettings launch scanapp via settings
func launchScanappViaSettings(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	ui := uiauto.New(tconn)

	entryFinder := nodewith.Name(apps.Scan.Name + " Scan documents and images").Role(role.Link).Ancestor(ossettings.WindowFinder)

	cr := s.PreValue().(*chrome.Chrome)
	// open printing page on settings
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrinting", ui.Exists(entryFinder)); err != nil {
		return errors.Wrap(err, "failed to launch Settings page")
	}

	// click scan link to open scanapp
	if err := ui.LeftClick(entryFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to click entry")
	}

	// wait for app visible
	if err := ash.WaitForApp(ctx, tconn, apps.Scan.ID, time.Minute); err != nil {
		return errors.Wrap(err, "could not find app in shelf after launch")
	}

	// close settings app
	if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
		return err
	}

	return nil
}

// launchScanappViaLauncher launch scanapp via launcher
func launchScanappViaLauncher(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	// create keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// launcher search and launch
	if err := launcher.SearchAndLaunch(tconn, kb, apps.Scan.Name)(ctx); err != nil {
		return errors.Wrap(err, "failed to search and launch scan app")
	}

	// wait for app visible
	if err := ash.WaitForApp(ctx, tconn, apps.Scan.ID, time.Minute); err != nil {
		return errors.Wrap(err, "could not find app in shelf after launch")
	}

	return nil
}

// verifyScanFile first: check file in folder ,second: transfer file to server then compare it
func verifyScanFile(ctx context.Context, s *testing.State, startTime time.Time, ts *testingStruct) error {

	var path string
	if ts.scanto == scantoMyFiles {
		path = filesapp.MyFilesPath
	} else {
		path = filesapp.DownloadPath
	}

	// pat = regexp.MustCompile(`^scan_\d{8}-\d{6}[^.]*\.pdf$`)
	var pat *regexp.Regexp
	pat = regexp.MustCompile(`^scan_\d{8}-\d{6}[^.]*\.` + regexp.QuoteMeta(strings.ToLower(string(ts.FileType))) + `$`)

	// file should be in folder
	fs, err := utils.WaitForFileSaved(ctx, path, pat, startTime)
	if err != nil {
		return errors.Wrap(err, "failed to wait for file saved")
	}

	scanfile := filepath.Join(path, fs.Name())

	// upload file to wwcb server
	filepath, err := utils.UploadFile(ctx, scanfile)
	if err != nil {
		return errors.Wrap(err, "failed to upload file to wwcb server")
	}

	// let wwcb server check pic
	if err := compareScannerPic(s,
		string(ts.ColorMode),
		string(ts.PageSize),
		string(ts.Resolution),
		filepath); err != nil {
		return err
	}

	return nil
}

// getScannerFromUI get scanner name from ui
func getScannerFromUI(ctx context.Context, s *testing.State, tconn *chrome.TestConn) (string, string, error) {

	s.Log("Getting scanner from ui ")

	var scannerUsb, scannerWifi string

	if err := testing.Poll(ctx, func(ctx context.Context) error {

		app, err := scanapp.Launch(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to launch scanapp")
		}

		testing.Sleep(ctx, time.Second)

		// open scanner dropdown
		dropdownFinder := nodewith.Name(string(scanapp.DropdownNameScanner)).ClassName("md-select")
		dropdownOptionFinder := nodewith.Role(role.ListBoxOption).First()
		if err := uiauto.Combine("Getting scanner from ui..",
			app.WaitUntilExists(dropdownFinder),
			app.LeftClick(dropdownFinder),
			app.WaitUntilExists(dropdownOptionFinder))(ctx); err != nil {
			return err
		}

		// find ui info
		params := ui.FindParams{
			Role: ui.RoleType(role.ListBoxOption),
		}
		scanners, err := ui.FindAll(ctx, tconn, params)
		if err != nil {
			return errors.Wrap(err, "failed to find scanners on ui")
		}

		app.Close(ctx)

		// scanners should have at least 2 device
		if len(scanners) < 2 {
			return errors.Errorf("failed to get enough scanner, got %d, want 2", len(scanners))
		}

		for _, scanner := range scanners {
			if strings.Contains(scanner.Name, "USB") {
				scannerUsb = scanner.Name
			} else {
				scannerWifi = scanner.Name
			}
		}

		if scannerUsb == "" || scannerWifi == "" {
			return errors.New("Scanner name should not be blank")
		}

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return "", "", errors.Wrap(err, "failed to get scanner from ui")
	}

	s.Logf("Found scanner(usb): %s, and scanner(wifi): %s", scannerUsb, scannerWifi)

	return scannerUsb, scannerWifi, nil
}

// setScanto set scanTo where
func setScanto(ctx context.Context, s *testing.State, tconn *chrome.TestConn, folder scanto) error {

	s.Logf("Setting scanto as %s", folder)

	scanToFinder := nodewith.Name(string(scanapp.DropdownNameScanTo)).ClassName("md-select")
	selectFolderFinder := nodewith.Name(string(scantoSelectFolder)).Role(role.ListBoxOption)
	folderFinder := nodewith.Name(string(folder)).Role(role.Button)
	openButtonFinder := nodewith.Name("Open").Role(role.Button)

	ui := uiauto.New(tconn)

	// click select folder in filesapp
	if err := uiauto.Combine("Checking..",
		ui.WaitUntilExists(scanToFinder),
		ui.LeftClick(scanToFinder),
		ui.WaitUntilExists(selectFolderFinder),
		ui.LeftClick(selectFolderFinder),
	)(ctx); err != nil {
		return err
	}

	testing.Sleep(ctx, 5*time.Second)

	// click to select folder
	if err := uiauto.Combine("Checking..",
		ui.WaitUntilExists(folderFinder),
		ui.DoubleClick(folderFinder),
	)(ctx); err != nil {
		return err
	}

	testing.Sleep(ctx, 5*time.Second)

	// click open button
	if err := uiauto.Combine("Checking..",
		ui.WaitUntilExists(openButtonFinder),
		ui.LeftClick(openButtonFinder),
	)(ctx); err != nil {
		return err
	}

	testing.Sleep(ctx, 5*time.Second)

	return nil
}

// confirmDetails check avahiBrowse result
func confirmDetails(ctx context.Context, s *testing.State, scannerInfo map[string]string, whatKind scanapp.DropdownName, whatThing string) error {
	switch whatKind {
	// color mode: grayscale, color
	// 	"cs": "grayscale,color",
	case scanapp.DropdownNameColorMode:
		s.Log(scannerInfo["cs"])
	//  file type: jpg, png, pdf
	// 	"pdl": "image/jpeg,application/pdf",
	case scanapp.DropdownNameFileType:
		s.Log(scannerInfo["pdl"])
	// source:
	// 	"duplex": "F",
	// 	"is": "platen,adf",
	case scanapp.DropdownNameSource:
		s.Log(scannerInfo["is"])
		s.Log(scannerInfo["duplex"])
	default:
		s.Log(scannerInfo["123"])
	}

	return nil
}

// avahiBrowse
// use command, then get return as follow
// {
// 	"UUID": "00000000-0000-1000-8000-0018dc00cb2b",
// 	"adminurl": "http://c797C9400000.local./index.html?page",
// 	"cs": "grayscale,color",
// 	"duplex": "F",
// 	"is": "platen,adf",
// 	"mopria-certified-scan": "1.3",
// 	"note": "",
// 	"pdl": "image/jpeg,application/pdf",
// 	"representation": "http://c797C9400000.local./icon/printer_icon.png",
// 	"rs": "eSCL",
// 	"txtvers": "1",
// 	"ty": "Canon TR4700 series",
// 	"usb_MFG": "Canon",
// 	"vers": "2.63"
// }
// func avahiBrowse(ctx context.Context, s *testing.State) (map[string]string, error) {
// 	cmd := testexec.CommandContext(ctx, "avahi-browse", "-t", "-r", "_uscans._tcp")
// 	out, err := cmd.Output(testexec.DumpLogOnError)
// 	if err != nil {
// 		return nil, err
// 	}
// 	lines := strings.Split(string(out), "\n")

// 	scanner := make(map[string]string)
// 	for _, line := range lines {
// 		// get line contain txt
// 		if strings.Contains(line, "txt") {
// 			group := strings.SplitN(strings.TrimSpace(line), "=", 2)
// 			// group = strings.ReplaceAll(group[1], "]", "")
// 			for _, item := range strings.Split(group[1], "\"") {
// 				if strings.Contains(item, "=") {
// 					keyValue := strings.Split(item, "=")
// 					scanner[keyValue[0]] = keyValue[1]
// 				}
// 			}

// 		}
// 	}

// 	s.Logf("Scanner info is %s", utils.PrettyPrint(scanner))

// 	return scanner, nil
// }

// compareScannerPic compare scanner pic
// compare_scan_pic(color:str ,size:str ,resolution:str ,filepath: str)
// color		size	resolution
// -----------------------------------
// color 		a4		75
// grayscale	letter	100
//				fit		150
//						200
//						300
//						600
// http://192.168.1.199:8585/api/compare_pic?color=color&size=a4&resolution=75&filepath=/filepath/on/tast'
func compareScannerPic(s *testing.State, color, size, resolution, filepath string) error {

	// regex to int
	intRes := regexp.MustCompile("[0-9]+").FindString(resolution)

	WWCBServerURL, ok := s.Var("FixtureWebURL")
	if !ok {
		return errors.Errorf("Runtime variable %s is not provided", WWCBServerURL)
	}

	// compare_pic api declare this
	if size == string(scanapp.PageSizeFitToScanArea) {
		size = "fit"
	}

	// construct URL
	URL := fmt.Sprintf("%s/api/compare_scan_pic?color=%s&size=%s&resolution=%s&file_path=%s",
		WWCBServerURL,
		strings.ToLower(color),
		strings.ToLower(size),
		intRes,
		filepath)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", URL)
	}
	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	s.Log("response: ", data)

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0" || m["resultTxt"] != "success" {
		return errors.New("failed to get correct response: ")
	}

	return nil
}
