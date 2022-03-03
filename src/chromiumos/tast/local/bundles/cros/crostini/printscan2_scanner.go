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
// ----Via Launcher - start typing Scan
// ----Via ‘Scan’ in Settings

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

package crostini

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
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

const (
	// first printer
	HpWifi = "HP ENVY Photo 7100 series [E4B456]"
	HpUsb  = "HP ENVY Photo 7100 series [USB]"

	// second printer
	CanonWifi = "Canon TR4700 series"
	CanonUsb  = "Canon TR4700 series (USB)"
)

const (
	Resolution100DPI scanapp.Resolution = "100 dpi"
	Resolution200DPI scanapp.Resolution = "200 dpi"
)

type MyScanSettings struct {
	Through    Through
	Via        Via
	Source     scanapp.Source
	Scanto     Scanto
	ColorMode  scanapp.ColorMode
	PageSize   scanapp.PageSize
	FileType   scanapp.FileType
	Resolution scanapp.Resolution
}

type Through string

const (
	ThroughUsb  = "USB"
	ThroughWifi = "WIFI"
)

type Via string

const (
	ViaLauncher Via = "Launcher"
	ViaSettings Via = "Settings"
)

type Scanto string

const (
	ScantoMyFiles      Scanto = "My files"
	ScantoDownloads    Scanto = "Downloads"
	ScantoSelectFolder Scanto = "Select folder in Files app…"
)

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

// reference scanning.go
func Printscan2Scanner(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// if err := testLoop(ctx, s, tconn); err != nil {
	// 	s.Fatal(err)
	// }

	// return
	// // 1. Connect a scanner device through either
	// // -WiFi
	// // -USB
	// if err := Printscan2Scanner_Step1(ctx, s, tconn); err != nil {
	// 	s.Fatal("Failed to execute step1: ")
	// }

	// // 2. Open ‘Scan’ Utility app or go to Settings-> Print and Scan-> Scan.
	// // ----Via Launcher - start typing Scan
	// // ----Via ‘Scan’ in Settings
	// if err := Printscan2Scanner_Step2(ctx, s, cr, tconn); err != nil {
	// 	s.Fatal("Failed to execute step2: ", err)
	// }

	// // 3. Observe the available selection options and choose:
	// // ----Scanning device from ‘Scanner’ (Default: Saved/Available Printers “A-Z”)
	// // ----Document feeder or Flatbed from ‘Source’(Default: Flatbed)
	// // *If one of these is missing, confirm the scanner device does not support it
	// // ----My Files or ‘Select folder in Files app…’ from ‘Scan to’,
	// //   *By clicking “Select folder in Files app…”, the Files app will open for selection (Downloads, Play files: Movies, Music, Pictures; Google Drive: My Drive; or New Folder)
	// // ----PDF, PNG, JPG from ‘File type’. (Default: PNG)
	// // 4. Observe the available selections for ‘More settings’ section and choose:
	// // ----Color or Grayscale from ‘Color mode’
	// // *If one of these is missing, confirm the scanner device does not support it
	// // ----Letter, A4 and Fit to scan Area
	// // ----75 dpi, 100 dpi, 200 dpi, 300 dpi, 600 dpi from ‘Resolution’. (Default: 300 dpi)
	// if err := Printscan2Scanner_Step3and4(ctx, s, tconn); err != nil {
	// 	s.Fatal("Failed to executep step 3, 4: ")
	// }

	// usb, wifi, err := getScannerFromUi(ctx, s, tconn)
	// if err != nil {
	// 	s.Fatal("Faile to get scanner from ui: ", err)
	// }

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
	if err := Printscan2Scanner_Step5and6(ctx, s, cr, tconn, CanonUsb, CanonWifi); err != nil {
		s.Fatal("Failed to execute step5, 6: ", err)
	}

}

// 1. Connect a scanner device through either
// -WiFi
// -USB
func Printscan2Scanner_Step1(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 1 - Connect a scanner device through either")

	s.Log("Wifi - connect maunauly in advanced")

	s.Log("USB - plug in usb")
	if err := utils.DoSwitchFixture(ctx, s, utils.UsbPrinterType, utils.UsbPrinterIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect usb: ")
	}

	// verfiy connected
	if _, err := ash.WaitForNotification(ctx, tconn, time.Minute, ash.WaitTitle("USB printer connected")); err != nil {
		s.Fatalf("Failed to wait for notification: %v", err)
	}

	return nil
}

// 2. Open ‘Scan’ Utility app or go to Settings-> Print and Scan-> Scan.
// ----Via Launcher - start typing Scan
// ----Via ‘Scan’ in Settings
func Printscan2Scanner_Step2(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 2 - Open ‘Scan’ Utility app or go to Settings-> Print and Scan-> Scan.")

	// launch scan via launcher
	if err := launchScanapp(ctx, s, tconn, ViaLauncher); err != nil {
		return errors.Wrapf(err, "Failed to launch scanapp via launcher: ")
	}

	// close scan app
	if err := apps.Close(ctx, tconn, apps.Scan.ID); err != nil {
		return err
	}

	// launch scan via settings
	if err := launchScanapp(ctx, s, tconn, ViaSettings); err != nil {
		return errors.Wrapf(err, "Failed to launch scanapp via settings: ")
	}

	// close scan app
	if err := apps.Close(ctx, tconn, apps.Scan.ID); err != nil {
		return err
	}

	return nil
}

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
func Printscan2Scanner_Step3and4(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 3, 4 - Observe the available selection options and choose")

	missings := make(map[string]string)

	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to launch app: ")
	}

	time.Sleep(time.Second)

	if err := app.ClickMoreSettings()(ctx); err != nil {
		return err
	}

	for _, dropdown := range []struct {
		name   scanapp.DropdownName
		option string
	}{
		{scanapp.DropdownNameSource, string(scanapp.SourceFlatbed)},
		{scanapp.DropdownNameSource, string(scanapp.SourceADFOneSided)},
		// {scanapp.DropdownNameScanTo, string(ScantoMyFiles)},
		// {scanapp.DropdownNameScanTo, string(ScantoSelectFolder)},
		{scanapp.DropdownNameFileType, string(scanapp.FileTypePNG)},
		{scanapp.DropdownNameFileType, string(scanapp.FileTypeJPG)},
		{scanapp.DropdownNameFileType, string(scanapp.FileTypePDF)},
		{scanapp.DropdownNameColorMode, string(scanapp.ColorModeColor)},
		{scanapp.DropdownNameColorMode, string(scanapp.ColorModeGrayscale)},
		{scanapp.DropdownNamePageSize, string(scanapp.PageSizeLetter)},
		{scanapp.DropdownNamePageSize, string(scanapp.PageSizeA4)},
		{scanapp.DropdownNamePageSize, string(scanapp.PageSizeFitToScanArea)},
		{scanapp.DropdownNameResolution, string(scanapp.Resolution75DPI)},
		{scanapp.DropdownNameResolution, string(Resolution100DPI)},
		{scanapp.DropdownNameResolution, string(scanapp.Resolution150DPI)},
		{scanapp.DropdownNameResolution, string(Resolution200DPI)},
		{scanapp.DropdownNameResolution, string(scanapp.Resolution300DPI)},
		{scanapp.DropdownNameResolution, string(scanapp.Resolution600DPI)},
	} {
		dropdownFinder := nodewith.Name(string(dropdown.name)).ClassName("md-select")
		dropdownOptionFinder := nodewith.Name(dropdown.option).Role(role.ListBoxOption)

		if err := uiauto.Combine("Checking..",
			app.WaitUntilExists(dropdownFinder),
			app.LeftClick(dropdownFinder),
			app.WaitUntilExists(dropdownOptionFinder),
			app.LeftClick(dropdownFinder),
		)(ctx); err != nil {
			missings[string(dropdown.name)] = dropdown.option
		}

	}

	app.Close(ctx)

	s.Logf("These are missing %s", prettyPrint(missings))

	return nil
}

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
func Printscan2Scanner_Step5and6(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, scannerUsb string, scannerWifi string) error {

	s.Logf("Step 5, 6 - Repeat above steps with different combinations ")

	flows := []MyScanSettings{}

	// Wi-Fi	Launcher	Flatbed,	My Files	Color	A4	PDF	75 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaLauncher,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: scanapp.Resolution75DPI,
	})
	// Wi-Fi	Launcher	Flatbed,	My Files	Grayscale	Letter	PNG	300 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaLauncher,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: scanapp.Resolution300DPI,
	})
	// Wi-Fi	Launcher	Document(One-sided), Feeder	Downloads	Color	Fit to scan area	JPG	150 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaLauncher,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution150DPI,
	})
	// Wi-Fi	Launcher	Document(One-sided), Feeder	Downloads	Grayscale	A4	PDF	200 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaLauncher,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: Resolution200DPI,
	})
	// Wi-Fi	Settings	Flatbed,	My Files	Color	Letter	PNG	100 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaSettings,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: Resolution100DPI,
	})
	// Wi-Fi	Settings	Flatbed,	My Files	Grayscale	Fit to scan area	JPG	600 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaSettings,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution600DPI,
	})
	// Wi-Fi	Settings	Document Feeder(One-sided),	Downloads	Color	A4	PDF	75 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaSettings,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: scanapp.Resolution75DPI,
	})
	// Wi-Fi	Settings	Document Feeder(One-sided),	 Downloads	Grayscale	Letter	PNG	100 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughWifi,
		Via:        ViaSettings,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: Resolution100DPI,
	})
	// USB	Launcher	Flatbed,	My Files	Color	Fit to scan area	JPG	150 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaLauncher,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution150DPI,
	})
	// USB	Launcher	Flatbed,	My Files	Grayscale	A4	PDF	200 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaLauncher,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: Resolution200DPI,
	})
	// USB	Launcher	Document Feeder(One-sided),	 Downloads	Color	Letter	PNG	300 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaLauncher,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: scanapp.Resolution300DPI,
	})
	// USB	Launcher	Document Feeder(One-sided),	 Downloads	Grayscale	Fit to scan area	JPG	600 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaLauncher,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution600DPI,
	})
	// USB	Settings	Flatbed,	My Files	Color	A4	PDF	75 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaSettings,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: scanapp.Resolution75DPI,
	})
	// USB	Settings	Flatbed,	My Files	Grayscale	Letter	PNG	100 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaSettings,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeLetter,
		FileType:   scanapp.FileTypePNG,
		Resolution: Resolution100DPI,
	})
	// USB	Settings	Document Feeder(One-sided),	 Downloads	Color	Fit to scan area	JPG	150 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaSettings,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeFitToScanArea,
		FileType:   scanapp.FileTypeJPG,
		Resolution: scanapp.Resolution150DPI,
	})
	// USB	Settings	Document Feeder(One-sided),	 Downloads	Grayscale	A4	PDF	200 dpi
	flows = append(flows, MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaSettings,
		Source:     scanapp.SourceADFOneSided,
		Scanto:     ScantoDownloads,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: Resolution200DPI,
	})

	for _, flow := range flows {

		s.Logf("%s", prettyPrint(flow))

		if err := runMyScanSettings(ctx, s, tconn, &flow, scannerUsb, scannerWifi); err != nil {
			return errors.Wrap(err, "Failed to run scan settings: ")
		}

		continue

		var scanner string
		// set scanner by which through which way
		if flow.Through == ThroughUsb {
			scanner = scannerUsb
		} else {
			scanner = scannerWifi
		}

		// according whichVia open scanner
		if err := launchScanapp(ctx, s, tconn, flow.Via); err != nil {
			return err
		}

		// create scanapp to use
		app, err := scanapp.Launch(ctx, tconn)
		if err != nil {
			return err
		}

		// set scanto
		if err := setScanto(ctx, s, tconn, flow.Scanto); err != nil {
			return err
		}

		// notice user put file into document position
		if flow.Source == scanapp.SourceADFOneSided {
			msg := fmt.Sprintf("Please input file into %s", string(scanapp.SourceADFOneSided))
			if err := utils.FixtureServerNotice(s, msg); err != nil {
				return err
			}
		}

		// set scan settings
		settings := scanapp.ScanSettings{
			Scanner:    scanner,
			Source:     flow.Source,
			FileType:   flow.FileType,
			ColorMode:  flow.ColorMode,
			PageSize:   flow.PageSize,
			Resolution: flow.Resolution,
		}

		if err := uiauto.Combine("scan",
			app.ClickMoreSettings(),
			app.SetScanSettings(settings),
			app.Scan(),
		)(ctx); err != nil {
			s.Fatal("Failed to perform scan: ", err)
		}

		app.Close(ctx)

	}
	return nil
}

func runMyScanSettings(ctx context.Context, s *testing.State, tconn *chrome.TestConn, flow *MyScanSettings, scannerUsb string, scannerWifi string) error {

	var scanner string
	if flow.Through == ThroughUsb {
		scanner = scannerUsb
	} else {
		scanner = scannerWifi
	}

	if err := launchScanapp(ctx, s, tconn, flow.Via); err != nil {
		return err
	}

	// launch app
	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to launch scan app: ")
	}

	// scan to
	if err := setScanto(ctx, s, tconn, flow.Scanto); err != nil {
		return errors.Wrap(err, "Failed to set scanto: ")
	}

	// when scan source is document one side,
	// notice user to put file
	if flow.Source == scanapp.SourceADFOneSided {
		msg := fmt.Sprintf("Please put file into %s", scanapp.SourceADFOneSided)
		if err := utils.FixtureServerNotice(s, msg); err != nil {
			return errors.Wrap(err, "Failed to show msg on server: ")
		}
	}

	// set settings and perform scan
	settings := scanapp.ScanSettings{
		Scanner:    scanner,
		Source:     flow.Source,
		FileType:   flow.FileType,
		ColorMode:  flow.ColorMode,
		PageSize:   flow.PageSize,
		Resolution: flow.Resolution,
	}
	if err := uiauto.Combine("Perform scan",
		app.ClickMoreSettings(),
		app.SetScanSettings(settings),
		app.WithTimeout(time.Minute).Scan(),
	)(ctx); err != nil {
		return errors.Wrap(err, "Failed to perform scan: ")
	}

	app.Close(ctx)

	return nil
}

// launch scanapp via launcher or settings
func launchScanapp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, whichVia Via) error {

	if whichVia == ViaSettings { // via settings
		if err := launchScanappViaSettings(ctx, s, tconn); err != nil {
			return errors.Wrap(err, "Failed to launch scanpp via settings: ")
		}
	} else { // via launcher
		if err := launchScanappViaLauncher(ctx, s, tconn); err != nil {
			return errors.Wrap(err, "Failed to launch scanpp via launcher: ")
		}
	}

	return nil
}

// launch scanapp via settings
func launchScanappViaSettings(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	// Val: settingsTestParams{
	// 	appID:        apps.Scan.ID,
	// 	menuLabel:    apps.Scan.Name + " Scan documents and images",
	// 	settingsPage: "osPrinting", // URL for Print and scan page
	// },
	ui := uiauto.New(tconn)

	entryFinder := nodewith.Name(apps.Scan.Name + " Scan documents and images").Role(role.Link).Ancestor(ossettings.WindowFinder)

	cr := s.PreValue().(*chrome.Chrome)
	// open printing page on settings
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrinting", ui.Exists(entryFinder)); err != nil {
		return errors.Wrapf(err, "Failed to launch Settings page: ")
	}

	// click scan link to open scanapp
	if err := ui.LeftClick(entryFinder)(ctx); err != nil {
		return errors.Wrapf(err, "Failed to click entry: ")
	}

	// wait for app visible
	if err := ash.WaitForApp(ctx, tconn, apps.Scan.ID, time.Minute); err != nil {
		return errors.Wrapf(err, "Could not find app in shelf after launch: ")
	}

	// close settings app
	if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
		return err
	}

	return nil
}

// launch scanapp via launcher
func launchScanappViaLauncher(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	// create keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrapf(err, "Failed to find keyboard: ")
	}
	defer kb.Close()

	// launcher search and launch
	if err := launcher.SearchAndLaunch(tconn, kb, apps.Scan.Name)(ctx); err != nil {
		return errors.Wrap(err, "Failed to search and launch scan app: ")
	}

	// wait for app visible
	if err := ash.WaitForApp(ctx, tconn, apps.Scan.ID, time.Minute); err != nil {
		return errors.Wrapf(err, "Could not find app in shelf after launch: ")
	}

	return nil
}

// first, check file in folder
// second, transfer file to server then compare it
func verifyScanFile(ctx context.Context, s *testing.State, startTime time.Time, flowSettings *MyScanSettings) error {

	var path string
	if flowSettings.Scanto == ScantoMyFiles {
		path = filesapp.MyFilesPath
	} else {
		path = filesapp.DownloadPath
	}

	// pat = regexp.MustCompile(`^scan_\d{8}-\d{6}[^.]*\.pdf$`)
	var pat *regexp.Regexp
	pat = regexp.MustCompile(`^scan_\d{8}-\d{6}[^.]*\.` + regexp.QuoteMeta(strings.ToLower(string(flowSettings.FileType))) + `$`)

	// file should be in folder
	fs, err := WaitForFileSaved(ctx, path, pat, startTime)
	if err != nil {
		return err
	}

	scanfile := filepath.Join(path, fs.Name())

	// copy file to tast
	// retrieve filename
	_, filename := filepath.Split(scanfile)

	// transfer file to tast env
	dir, ok := testing.ContextOutDir(ctx)
	if ok && dir != "" {
		if _, err := os.Stat(dir); err == nil {
			testing.ContextLogf(ctx, "copy file to %s", dir)

			// read file
			b, err := ioutil.ReadFile(scanfile)
			if err != nil {
				return err
			}

			// write tastPath to result folder
			tastPath := filepath.Join(s.OutDir(), filename)
			if err := ioutil.WriteFile(tastPath, b, 0644); err != nil {
				return errors.Wrapf(err, "failed to dump bytes to %s", tastPath)
			}
		}
	}

	// verify file by allion utils func
	// if err:=allonutils.ComparePic(f)
	tastPath := filepath.Join(utils.GetOutputPath(s), filename)
	if err := utils.CompareScannerPic(s,
		string(flowSettings.ColorMode),
		string(flowSettings.PageSize),
		string(flowSettings.Resolution),
		tastPath); err != nil {
		return err
	}

	return nil
}

// change timeout from 5s to 60s
// refer to cca.go in pacakge cca
// WaitForFileSaved waits for the presence of the captured file with file name matching the specified
// pattern, size larger than zero, and modified time after the specified timestamp.
func WaitForFileSaved(ctx context.Context, dir string, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	const timeout = time.Minute
	var result os.FileInfo
	seen := make(map[string]struct{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read the camera directory")
		}
		for _, file := range files {
			if file.Size() == 0 || file.ModTime().Before(ts) {
				continue
			}
			if _, ok := seen[file.Name()]; ok {
				continue
			}
			seen[file.Name()] = struct{}{}
			testing.ContextLog(ctx, "New file found: ", file.Name())
			if pat.MatchString(file.Name()) {
				testing.ContextLog(ctx, "Found a match: ", file.Name())
				result = file
				return nil
			}
		}
		return errors.New("no matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, errors.Wrapf(err, "no matching output file found after %v", timeout)
	}
	return result, nil
}

// get scanner name from ui
func getScannerFromUi(ctx context.Context, s *testing.State, tconn *chrome.TestConn) (string, string, error) {

	s.Logf("Getting scanner from ui ..")

	var scannerUsb, scannerWifi string

	if err := testing.Poll(ctx, func(ctx context.Context) error {

		app, err := scanapp.Launch(ctx, tconn)
		if err != nil {
			return errors.Wrapf(err, "Failed to launch scanapp: ")
		}

		// let scan app get ready
		time.Sleep(time.Second)

		// open scanner dropdown
		dropdownFinder := nodewith.Name(string(scanapp.DropdownNameScanner)).ClassName("md-select")
		dropdownOptionFinder := nodewith.Role(role.ListBoxOption).First()
		if err := uiauto.Combine("Getting scanner from ui..",
			app.WaitUntilExists(dropdownFinder),
			app.LeftClick(dropdownFinder),
			app.WaitUntilExists(dropdownOptionFinder))(ctx); err != nil {
			return err
		}

		// time.Sleep(time.Second)

		// find ui info
		params := ui.FindParams{
			Role: ui.RoleType(role.ListBoxOption),
		}
		scanners, err := ui.FindAll(ctx, tconn, params)
		if err != nil {
			return errors.Wrapf(err, "Failed to find scanners on ui: ")
		}

		// scanners should have at least 2 device
		if len(scanners) < 2 {
			return errors.Errorf("Failed to get enough scanner, got %d, want 2", len(scanners))
		}

		for _, scanner := range scanners {
			if strings.Contains(scanner.Name, "USB") {
				scannerUsb = scanner.Name
			} else {
				scannerWifi = scanner.Name
			}
		}

		if scannerUsb == "" || scannerWifi == "" {
			return errors.Errorf("Scanner name should not be blank")
		}

		app.Close(ctx)

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return "", "", errors.Wrapf(err, "Failed to get scanner from ui:")
	}

	s.Logf("Found scanner(usb): %s, and scanner(wifi): %s", scannerUsb, scannerWifi)

	return scannerUsb, scannerWifi, nil
}

// set scanto where
func setScanto(ctx context.Context, s *testing.State, tconn *chrome.TestConn, folder Scanto) error {

	s.Logf("Setting scanto as %s", folder)

	scanToFinder := nodewith.Name(string(scanapp.DropdownNameScanTo)).ClassName("md-select")
	selectFolderFinder := nodewith.Name(string(ScantoSelectFolder)).Role(role.ListBoxOption)
	folderFinder := nodewith.Name(string(folder)).Role(role.Button)
	openButtonFinder := nodewith.Name("Open").Role(role.Button)

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Checking..",
		ui.WaitUntilExists(scanToFinder),
		ui.LeftClick(scanToFinder),
		ui.WaitUntilExists(selectFolderFinder),
		ui.LeftClick(selectFolderFinder),
	)(ctx); err != nil {
		return err
	}

	// time.Sleep(5 * time.Second)

	if err := uiauto.Combine("Checking..",
		ui.WaitUntilExists(folderFinder),
		ui.DoubleClick(folderFinder),
	)(ctx); err != nil {
		return err
	}

	// time.Sleep(5 * time.Second)

	if err := uiauto.Combine("Checking..",
		ui.WaitUntilExists(openButtonFinder),
		ui.LeftClick(openButtonFinder),
	)(ctx); err != nil {
		return err
	}

	return nil
}

func confirmDetails(ctx context.Context, s *testing.State, scannerInfo map[string]string, whatKind scanapp.DropdownName, whatThing string) error {
	switch whatKind {
	// color mode: grayscale, color
	// 	"cs": "grayscale,color",
	case scanapp.DropdownNameColorMode:
		s.Logf(scannerInfo["cs"])
	//  file type: jpg, png, pdf
	// 	"pdl": "image/jpeg,application/pdf",
	case scanapp.DropdownNameFileType:
		s.Logf(scannerInfo["pdl"])
	// source:
	// 	"duplex": "F",
	// 	"is": "platen,adf",
	case scanapp.DropdownNameSource:
		s.Logf(scannerInfo["is"])
		s.Logf(scannerInfo["duplex"])
	default:
		s.Logf(scannerInfo["123"])
	}

	return nil
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

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
func avahiBrowse(ctx context.Context, s *testing.State) (map[string]string, error) {
	cmd := testexec.CommandContext(ctx, "avahi-browse", "-t", "-r", "_uscans._tcp")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")

	scanner := make(map[string]string)
	for _, line := range lines {
		// get line contain txt
		if strings.Contains(line, "txt") {
			group := strings.SplitN(strings.TrimSpace(line), "=", 2)
			// group = strings.ReplaceAll(group[1], "]", "")
			for _, item := range strings.Split(group[1], "\"") {
				if strings.Contains(item, "=") {
					keyValue := strings.Split(item, "=")
					scanner[keyValue[0]] = keyValue[1]
				}
			}

		}
	}

	s.Logf("Scanner info is %s", prettyPrint(scanner))

	return scanner, nil
}

func scanThenObserve(ctx context.Context, s *testing.State, tconn *chrome.TestConn, app *scanapp.ScanApp, flowSettings *MyScanSettings) error {
	// easy to fail , cuz is dynamic
	startTime := time.Now()

	s.Log("Start scanning & observe")

	if err := app.Scan()(ctx); err != nil {
		return err
	}

	return nil

	var scanButtonFinder *nodewith.Finder = nodewith.Name("Scan").Role(role.Button)
	var scanningPage *nodewith.Finder = nodewith.NameContaining("Scanning page")
	var scanningInProgress *nodewith.Finder = nodewith.NameContaining("Cancel").Role(role.Button)
	var scanningCompleted *nodewith.Finder = nodewith.NameContaining("Scanning completed").ClassName("preview")

	// ui := uiauto.New(tconn)
	if err := uiauto.Combine("Start scanning and observe",
		app.LeftClick(scanButtonFinder),
		app.WaitUntilExists(scanningPage),
		app.WaitUntilExists(scanningInProgress),
		app.WaitUntilExists(scanningCompleted),
	)(ctx); err != nil {
		return err
	}

	return nil

	// ----File should be in the location that the ‘Scan to’ attribute is set, in the intended format chosen from the File type attribute.
	if err := verifyScanFile(ctx, s, startTime, flowSettings); err != nil {
		return err
	}

	return nil
}

func testLoop(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {
	flow := MyScanSettings{
		Through:    ThroughUsb,
		Via:        ViaLauncher,
		Source:     scanapp.SourceFlatbed,
		Scanto:     ScantoMyFiles,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		FileType:   scanapp.FileTypePDF,
		Resolution: scanapp.Resolution75DPI,
	}

	for i := 0; i < 16; i++ {
		if err := runMyScanSettings(ctx, s, tconn, &flow, CanonUsb, CanonWifi); err != nil {
			s.Logf("%s", err)
		}
	}

	return nil
}
