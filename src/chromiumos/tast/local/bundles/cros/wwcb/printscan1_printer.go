// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// "Pre-Condition:
// ChromeOS device and USB connected printer with paper and sufficient printing ink or toner.

// Procedure:
// 1. Login to a ChromeOS device and connect USB printer.
// 2. Open google search page and trigger print dialog on the current chrome page by pressing Ctrl + p.
// 3. Dropdown arrow next to 'Destination' Printer should be on the list if not, select on see more (default is 'Save as PDF').
// ---- 'Select A destination' pop-up window should contain contain the printer name.
// 3.a. If the USB printer name is not displayed in list of printers , add the printer manually, e.g. ""settings-> Print and Scan->printers->add printer-> e.g EPSON XP-430 (USB)""
// ---- After triggering Print dialog, the list of printers should contain the added printer name.

// 4. Select the printer and proceed to printing with this printer (e.g. HP OfficeJet 4650)
// ---- Verify printer produces the printed page successfully(limited to reasonable acceptance)
// ---- Verify print job status notification is present

// 5. Start a print job and unplug the USB connection in the middle of the print job.
// ---- Verify print job failed notification should be present.
// ---- ChromeOS device should not hang/crash.
// ---- Another print job should be executed successfully.

// 6. Confirm PDF and PNG files are printed successfully

// 7. Confirm print job can be done from ARC++ / Android app ( e.g. Chrome, gDocs, MSword,etc.)
// ---- the same USB printer (as set in chrome://settings) can be used
// ---- no extra steps in the printing steps sequence
// ---- no impact to system stability or quality of the printed page

// 8. Confirm printing with changes to 'More settings' controls works as intended. Like:
// - scale
// - Pages per sheet
// - Paper size
// - Quality
// - Options: Headers and footers; Two-sided; Background graphics   "

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/printpreview"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	defaultTimeout = 20 * time.Second
)

// comparePrinterKey compare printer key
type comparePrinterKey string

// compare printer option
const (
	comparePrinterPDF                comparePrinterKey = "pdf"
	comparePrinterPNG                comparePrinterKey = "png"
	comparePrinterScale              comparePrinterKey = "scale"
	comparePrinterPapersize          comparePrinterKey = "size"
	comparePrinterPagespersheet      comparePrinterKey = "pages"
	comparePrinterQuality            comparePrinterKey = "quality"
	comparePrinterHeardersandfooters comparePrinterKey = "headers"
	comparePrinterBackgroundgraphics comparePrinterKey = "background"
)

type dropdownName string

const (
	dropdownScale        dropdownName = "Scale"
	dropdownPagePerSheet dropdownName = "Pages per sheet"
	dropdownMargins      dropdownName = "Margins"
	dropdownPaperSize    dropdownName = "Paper size"
	dropdownQuality      dropdownName = "Quality"
)

type paperSize string

const (
	paperSizeA0      paperSize = "A0"
	paperSizeA1      paperSize = "A1"
	paperSizeA2      paperSize = "A2"
	paperSizeA3      paperSize = "A3"
	paperSizeA4      paperSize = "A4"
	paperSizeLegal   paperSize = "Legal"
	paperSizeLetter  paperSize = "Letter"
	paperSizeTabloid paperSize = "Tabloid"
)

type pagePerSheet string

const (
	pagePerSheet1  pagePerSheet = "1"
	pagePerSheet2  pagePerSheet = "2"
	pagePerSheet4  pagePerSheet = "4"
	pagePerSheet6  pagePerSheet = "6"
	pagePerSheet9  pagePerSheet = "9"
	pagePerSheet16 pagePerSheet = "16"
)

type margins string

const (
	marginsDefault margins = "Default"
	marginsNone    margins = "None"
	marginsMinimum margins = "Minimum"
	marginsCustom  margins = "Custom"
)

type scale string

const (
	scaleCustom  scale = "Custom"
	scaleDefault scale = "Default"
)

type printType string

const (
	printTypeBROWSER printType = "BROWSER"
	printTypePDF     printType = "PDF"
	printTypePNG     printType = "PNG"
)

type printQuality string

const (
	printQualityHigh   printQuality = "High"
	printQualityNormal printQuality = "Normal"
)

type printOption string

const (
	printOptionHeadersandfooters  printOption = "Headers and footers"
	printOptionBackgroundgraphics printOption = "Background graphics"
)

// file store on wwcb server
const (
	printFilePDF = "Chrome_Letter.pdf"
	printFilePNG = "Chrome_Letter.png"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Printscan1Printer,
		Desc:     "Test USB printing from ChromeOS device",
		Contacts: []string{"allion-sw@allion.com"},
		// below params -> reference to launcher_apps.go
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},

		Timeout: time.Hour,
		VarDeps: []string{"ui.gaiaPoolDefault", "FixtureWebUrl"},
		Data:    []string{printFilePDF, printFilePNG},
	})
}

func Printscan1Printer(ctx context.Context, s *testing.State) {

	// Login option
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Setup test connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// 1. Login to a ChromeOS device and connect USB printer.
	if err := printscan1PrinterStep1(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step1: ", err)
	}

	// 2. Open google search page and trigger print dialog on the current chrome page by pressing Ctrl + p.
	// 3. Dropdown arrow next to 'Destination' Printer should be on the list if not, select on see more (default is 'Save as PDF').
	// ---- 'Select A destination' pop-up window should contain contain the printer name.
	// 3.a. If the USB printer name is not displayed in list of printers , add the printer manually, e.g. ""settings-> Print and Scan->printers->add printer-> e.g EPSON XP-430 (USB)""
	// ---- After triggering Print dialog, the list of printers should contain the added printer name.
	// 4. Select the printer and proceed to printing with this printer (e.g. HP OfficeJet 4650)
	// ---- Verify printer produces the printed page successfully(limited to reasonable acceptance)
	// ---- Verify print job status notification is present
	if err := printscan1PrinterStep2To4(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step2, 3, 4: ", err)
	}

	// 5. Start a print job and unplug the USB connection in the middle of the print job.
	// ---- Verify print job failed notification should be present.
	// ---- ChromeOS device should not hang/crash.
	// ---- Another print job should be executed successfully.

	// 6. Confirm PDF and PNG files are printed successfully
	if err := printscan1PrinterStep6(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	// 7. Confirm print job can be done from ARC++ / Android app ( e.g. Chrome, gDocs, MSword,etc.)
	// ---- the same USB printer (as set in chrome://settings) can be used
	// ---- no extra steps in the printing steps sequence
	// ---- no impact to system stability or quality of the printed page
	if err := printscan1PrinterStep7(ctx, s, cr); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	// 8. Confirm printing with changes to 'More settings' controls works as intended. Like:
	// - scale
	// - Pages per sheet
	// - Paper size
	// - Quality
	// - Options: Headers and footers; Two-sided; Background graphics   "
	if err := printscan1PrinterStep8(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
}

func printscan1PrinterStep1(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 1 - Login to a ChromeOS device and connect USB printer")

	// connect usb printer
	if err := utils.ControlFixture(ctx, s, utils.USBPrinterType, utils.USBPrinterIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect printer")
	}

	// verfiy connected
	if _, err := ash.WaitForNotification(ctx, tconn, time.Minute, ash.WaitTitle("USB printer connected")); err != nil {
		return errors.Wrap(err, "failed to wait for notification")
	}

	return nil
}

func printscan1PrinterStep2To4(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 2 - Open google search page and trigger print dialog on the current chrome page by pressing Ctrl + p")

	// Open browser window.
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		return errors.Wrap(err, "failed to open browser window")
	}
	defer conn.Close()

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// ctrl + p to trigger print dialog
	if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
		return errors.Wrap(err, "failed to press Ctrl+P to trigger print dialog")
	}

	s.Log("Step 3, 4 - Select printer and proceed to printing with this printer")

	if err := selectPrinter(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to select printer")
	}

	// Hide all notifications to prevent them from covering the print button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close all notifications")
	}

	printpreview.WaitForPrintPreview(tconn)(ctx)

	// start print
	if err := printpreview.Print(ctx, tconn); err != nil {
		return err
	}

	// wait for print completed
	if err := waitForPrintCompleted(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to")
	}

	// ctrl + w to close chrome
	if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
		return errors.Wrap(err, "failed to press Ctrl+W to close chrome")
	}

	return nil
}

func printscan1PrinterStep5(ctx context.Context, s *testing.State) error {

	s.Log("Step 5 - Start a print job and unplug the USB connection in the middle of the print job")

	// unplug usb

	// ---- Verify print job failed notification should be present.

	// ---- ChromeOS device should not hang/crash.
	// ---- Another print job should be executed successfully.

	return nil

}

func printscan1PrinterStep6(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 6 - Confirm PDF and PNG files are printed successfully")

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// copy pdf to downloads
	pdfFileLocation := filepath.Join(filesapp.DownloadPath, printFilePDF)
	if err := fsutil.CopyFile(s.DataPath(printFilePDF), pdfFileLocation); err != nil {
		s.Fatalf("Failed to copy the test pdf file to %s: %s", pdfFileLocation, err)
	}
	defer os.Remove(pdfFileLocation)

	// copy png to downloads
	pngFileLocation := filepath.Join(filesapp.DownloadPath, printFilePNG)
	if err := fsutil.CopyFile(s.DataPath(printFilePNG), pngFileLocation); err != nil {
		s.Fatalf("Failed to copy the test png file to %s: %s", pngFileLocation, err)
	}
	defer os.Remove(pngFileLocation)

	for _, testParams := range []struct {
		key  comparePrinterKey
		file string
	}{
		{comparePrinterPDF, printFilePDF},
		{comparePrinterPNG, printFilePNG},
	} {

		s.Logf("Confirm %s are printed successfully", testParams.file)

		// open file in download folder
		if err := openDownloadsFile(ctx, s, tconn, testParams.file); err != nil {
			return errors.Wrap(err, "failed to open file in downloads")
		}

		// ctrl + p to trigger print dialog
		if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
			return errors.Wrap(err, "failed to press Ctrl+P to trigger print dialog")
		}

		printpreview.WaitForPrintPreview(tconn)(ctx)

		// Hide all notifications to prevent them from covering the print button.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}

		// // print
		// if err := printpreview.Print(ctx, tconn); err != nil {
		// 	return errors.Wrap(err, "Failed to press print")
		// }

		// use enter instead, cuz screen has two print button
		if err := kb.Accel(ctx, "enter"); err != nil {
			return errors.Wrap(err, "failed to type enter")
		}

		// wait print job completed
		if err := waitForPrintCompleted(ctx, s, tconn); err != nil {
			return errors.Wrap(err, "failed to wait print job completed")
		}

		// verify print file
		if err := verifyPrintFile(ctx, s, tconn, testParams.key); err != nil {
			return errors.Wrap(err, "failed to verify print file")
		}

		// ctrl + w to close file
		if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
			return errors.Wrap(err, "failed to press Ctrl+W to close file")
		}

	}

	return nil
}

func printscan1PrinterStep7(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {

	s.Log("Step 7 - Confirm print job can be done from ARC++ / Android app")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// install docs app from play store
	const (
		pkgName = "com.picsel.tgv.app.smartoffice"
		actName = "com.artifex.sonui.ExplorerActivity"
		appName = "SmartOffice"
	)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	maxAttempts := 1

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		return errors.Wrap(err, "failed to optin to Play store")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {

		return errors.Wrap(err, "failed to wait for Play store")
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		return errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close(ctx)
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			}
			if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, -1); err != nil {
		return errors.Wrap(err, "failed to install app")
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to close playstore")
	}

	openAppCommand := testexec.CommandContext(ctx, "adb", "shell", "am", "start", "-n", pkgName+"/"+actName)
	if err := openAppCommand.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to start companion Android app using adb")
	}

	// Click on allow
	allowText := "ALLOW"
	allowClass := "android.widget.Button"
	allowButton := d.Object(ui.ClassName(allowClass), ui.TextMatches(allowText))
	if err := allowButton.WaitForExists(ctx, defaultTimeout); err != nil {
		return errors.Wrap(err, "allowButton doesn't exists")
	}
	if err := allowButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on allowButton")
	}

	// Click on download
	downloadText := "Download"
	downloadClass := "android.widget.TextView"
	downloadTextView := d.Object(ui.ClassName(downloadClass), ui.TextMatches(downloadText))
	if err := downloadTextView.WaitForExists(ctx, defaultTimeout); err != nil {
		s.Log("downloadTextView doesn't exists: ")
	} else if err := downloadTextView.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on downloadTextView")
	}

	// Click on file
	fileText := printFilePDF
	fileClass := "android.widget.TextView"
	fileTextView := d.Object(ui.ClassName(fileClass), ui.TextMatches(fileText))
	if err := fileTextView.WaitForExists(ctx, defaultTimeout); err != nil {
		s.Log("fileTextView doesn't exists: ")
	} else if err := fileTextView.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on fileTextView")
	}

	// Click on print
	printButton := d.Object(ui.ResourceID("com.picsel.tgv.app.smartoffice:id/print_button"))
	if err := printButton.WaitForExists(ctx, defaultTimeout); err != nil {
		s.Log("printButton doesn't exists: ")
	} else if err := printButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click printButton")
	}

	// Hide all notifications to prevent them from covering the print button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close all notifications")
	}

	// Wait for print preview to load before starting the print job.
	s.Log("Waiting for print preview to load")
	printpreview.WaitForPrintPreview(tconn)(ctx)

	// enter print
	// if err := printpreview.Print(ctx, tconn); err != nil {
	// 	return errors.Wrap(err, "Failed to press print: ")
	// }

	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	// wait print job completed
	if err := waitForPrintCompleted(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to wait print job completed")
	}

	// verify print file
	if err := verifyPrintFile(ctx, s, tconn, comparePrinterPDF); err != nil {
		return errors.Wrap(err, "failed to verify print file")
	}

	closeAppCommand := testexec.CommandContext(ctx, "adb", "shell", "am", "force-stop", pkgName)
	if err := closeAppCommand.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to close Android app using adb")
	}

	return nil
}

func printscan1PrinterStep8(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	// seq	name				origin  -> set to 	-> back to origin
	// 1. 	scale: 				100 	-> 50		-> 100
	// 2. 	Pages per sheet: 	1 		-> 2		-> 1
	// 3.	Paper size:  		letter 	-> A4		-> letter
	// 4.	Quality: 			normal	-> Hight	-> normal
	// 5.	Headers and footers true	-> true		-> true
	// 6.	Background graphics false 	-> true		-> true

	// var browserNode *nodewith.Finder = nodewith.Role(role.Window).First()

	s.Log("Step 8 - Confirm printing with changes to 'More settings' controls works as intended")

	// Open browser window.
	_, err := cr.NewConn(ctx, "")
	if err != nil {
		return errors.Wrap(err, "failed to open browser window")
	}
	// defer conn.Close()

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to kind keyboard")
	}
	defer kb.Close()

	for _, testParams := range []struct {
		key comparePrinterKey
	}{
		{comparePrinterScale},
		{comparePrinterPagespersheet},
		{comparePrinterPapersize},
		{comparePrinterQuality},
		{comparePrinterHeardersandfooters},
		{comparePrinterBackgroundgraphics},
	} {

		// ctrl + p to trigger print dialog
		if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
			return errors.Wrap(err, "failed to press Ctrl+P to trigger print dialog")
		}

		printpreview.WaitForPrintPreview(tconn)(ctx)

		// show more settings
		if err := showMoreSettingsVisible(ctx, s, tconn); err != nil {
			return nil
		}

		printpreview.WaitForPrintPreview(tconn)(ctx)

		// change
		switch testParams.key {

		case comparePrinterScale: // scale default -> custom 50

			// set scale to 50
			if err := setScale(ctx, tconn, "50"); err != nil {
				return err
			}

		case comparePrinterPagespersheet: // pages per sheet 1->2

			// change back
			if err := setDropdown(ctx, s, tconn, dropdownScale, string(scaleDefault)); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			// set sheet to 2
			if err := setDropdown(ctx, s, tconn, dropdownPagePerSheet, string(pagePerSheet2)); err != nil {
				return err
			}

		case comparePrinterPapersize: // paper size: letter -> a4

			// change back
			if err := setDropdown(ctx, s, tconn, dropdownPagePerSheet, string(pagePerSheet1)); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			// if err := utils.WebNotification(ctx, s, "Please change printer page size to A4"); err != nil {
			// 	return err
			// }

			if err := setDropdown(ctx, s, tconn, dropdownPaperSize, string(paperSizeA4)); err != nil {
				return err
			}

		case comparePrinterQuality: // quality: normal -> high

			// if err := utils.WebNotification(ctx, s, "Please change printer page size to Letter"); err != nil {
			// 	return err
			// }

			// change back
			if err := setDropdown(ctx, s, tconn, dropdownPaperSize, string(paperSizeLetter)); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			// set quality
			if err := setQuality(ctx, s, tconn, printQualityHigh); err != nil {
				return err
			}

		case comparePrinterHeardersandfooters: // headers: true -> false

			// change back
			if err := setQuality(ctx, s, tconn, printQualityNormal); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			// set headers and footers false
			if err := setOption(ctx, s, tconn, printOptionHeadersandfooters, checked.False); err != nil {
				return err
			}

		case comparePrinterBackgroundgraphics: // backgroud: false -> true

			// change back
			if err := setOption(ctx, s, tconn, printOptionHeadersandfooters, checked.True); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			// set backgroud graphics true
			if err := setOption(ctx, s, tconn, printOptionBackgroundgraphics, checked.True); err != nil {
				return err
			}

		}

		// Hide all notifications to prevent them from covering the print button.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}

		// print
		printpreview.WaitForPrintPreview(tconn)(ctx)

		if err := printpreview.Print(ctx, tconn); err != nil {
			return err
		}

		if err := waitForPrintCompleted(ctx, s, tconn); err != nil {
			return err
		}

		if err := verifyPrintFile(ctx, s, tconn, testParams.key); err != nil {
			return err
		}

	}

	return nil
}

// showMoreSettingsVisible when "more settings" is collapsed, click on it
func showMoreSettingsVisible(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()

	moreSettingsFinder := nodewith.Name("More settings").Role(role.Button)

	ui := uiauto.New(tconn)

	if err := ui.WaitForLocation(moreSettingsFinder)(ctx); err != nil {
		return err
	}

	nodeInfo, err := ui.Info(ctx, moreSettingsFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the more settings")
	}

	if nodeInfo.State[state.Collapsed] == true {
		// click more settings
		if err := ui.LeftClick(moreSettingsFinder)(ctx); err != nil {
			return err
		}
	}

	testing.Sleep(ctx, 2*time.Second)

	// move to bottom
	if err := kb.Accel(ctx, "search+right"); err != nil {
		return errors.Wrap(err, "failed to type end")
	}

	return nil
}

// setDropdown set drop down , according dropdown's name ,select dropdown option
func setDropdown(ctx context.Context, s *testing.State, tconn *chrome.TestConn, dropdownName dropdownName, dropdownOption string) error {

	s.Logf("Setting dropdown %s to %s ", dropdownName, dropdownOption)

	dropdownFinder := nodewith.Name(string(dropdownName)).ClassName("md-select")
	dropdownOptionFinder := nodewith.Name(dropdownOption).Role(role.ListBoxOption)

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Checking..",
		ui.WaitUntilExists(dropdownFinder),
		ui.WaitForLocation(dropdownFinder),
		ui.MakeVisible(dropdownFinder),
		ui.LeftClick(dropdownFinder),
		ui.WaitUntilExists(dropdownOptionFinder),
		ui.WaitForLocation(dropdownOptionFinder),
		ui.MakeVisible(dropdownOptionFinder),
		ui.LeftClick(dropdownOptionFinder))(ctx); err != nil {
		return err
	}

	testing.Sleep(ctx, time.Second)

	return nil
}

// setScale reference to function - SetPages
func setScale(ctx context.Context, tconn *chrome.TestConn, scales string) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()

	// convert to int
	amount, err := strconv.Atoi(scales)
	if err != nil {
		return err
	}

	// restrict scales range
	if int64(amount) < 10 || int64(amount) > 200 {
		return errors.New("Scale amount must be a number between 10 and 200")
	}

	// Find and expand the scale list.
	scaleList := nodewith.Name("Scale").Role(role.PopUpButton)
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("find and click scale list",
		ui.WithTimeout(10*time.Second).WaitUntilExists(scaleList),
		ui.LeftClick(scaleList),
	)(ctx); err != nil {
		return err
	}

	// Find the custom pages option to verify the pages list has expanded.
	customOption := nodewith.Name(string(scaleCustom)).Role(role.ListBoxOption)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(customOption)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for pages list to expand")
	}

	// Select "Custom" and set the desired page range.
	if err := kb.Accel(ctx, "search+right"); err != nil {
		return errors.Wrap(err, "failed to type end")
	}

	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	// Wait for the custom pages text field to appear and become focused (this
	// happens automatically).
	intField := nodewith.Name("100").Role(role.GenericContainer).State(state.Focusable, true)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(intField)(ctx); err != nil {
		return errors.Wrap(err, "failed to find custom pages text field")
	}

	testing.Sleep(ctx, time.Second)

	if err := kb.Type(ctx, scales); err != nil {
		return errors.Wrap(err, "failed to type pages")
	}

	return nil
}

// setQuality advanced settings -> search "Print quality" -> set quality
func setQuality(ctx context.Context, s *testing.State, tconn *chrome.TestConn, wantQuality printQuality) error {

	// Select "Custom" and set the desired page range.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	// click advanced settings
	advancedSettings := nodewith.Name("Advanced settings").Role(role.Button)
	if err := uiauto.Combine("Set quality ..",
		ui.WaitUntilExists(advancedSettings),
		ui.WaitForLocation(advancedSettings),
		ui.MakeVisible(advancedSettings),
		ui.LeftClick(advancedSettings))(ctx); err != nil {
		return err
	}

	// search print quality
	if err := kb.Type(ctx, "Print quality"); err != nil {
		return errors.Wrap(err, "failed to type end")
	}

	// set quality
	dropdownFinder := nodewith.ClassName("md-select").First()
	dropdownOptionFinder := nodewith.Name(string(wantQuality)).Role(role.ListBoxOption)
	applyFinder := nodewith.Name("Apply").Role(role.Button)
	if err := uiauto.Combine("Set quality ..",
		ui.WaitUntilExists(dropdownFinder),
		ui.LeftClick(dropdownFinder),
		ui.WaitUntilExists(dropdownOptionFinder),
		ui.LeftClick(dropdownOptionFinder),
		ui.LeftClick(applyFinder))(ctx); err != nil {
		return err
	}
	return nil
}

// setOption when option checked is not eqaul expected,do the click
func setOption(ctx context.Context, s *testing.State, tconn *chrome.TestConn, optionName printOption, checked checked.Checked) error {

	s.Logf("Setting option %s to %s ", string(optionName), string(checked))

	optionFinder := nodewith.Name(string(optionName)).Role(role.CheckBox)

	ui := uiauto.New(tconn)
	// Find the node info for the mirror checkbox.
	nodeInfo, err := ui.Info(ctx, optionFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the mirror checkbox")
	}
	// When not eqaul expected, to the click
	if nodeInfo.Checked != checked {
		testing.ContextLogf(ctx, "Click %q checkbox", optionName)

		if err := ui.MakeVisible(optionFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to make visible")
		}

		if err := ui.LeftClick(optionFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}
	}

	return nil
}

// launchPrintjobs launch "print jobs" through settings page
func launchPrintjobs(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	// Name: "print_management",
	// Val: settingsTestParams{
	// 	appID:        apps.PrintManagement.ID,
	// 	menuLabel:    apps.PrintManagement.Name + " View and manage print jobs",
	// 	settingsPage: "osPrinting", // URL for Print and scan page
	// },
	ui := uiauto.New(tconn)
	entryFinder := nodewith.Name(apps.PrintManagement.Name + " View and manage print jobs").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrinting", ui.Exists(entryFinder)); err != nil {
		return errors.Wrap(err, "failed to launch Settings page")
	}

	if err := ui.LeftClick(entryFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to click entry")
	}

	err := ash.WaitForApp(ctx, tconn, apps.PrintManagement.ID, time.Minute)
	if err != nil {
		return errors.Wrap(err, "could not find app in shelf after launch")
	}

	return nil
}

// openDownloadsFile open file in download folder
func openDownloadsFile(ctx context.Context, s *testing.State, tconn *chrome.TestConn, fileName string) error {

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch the Files App")
	}

	openButton := nodewith.Name("Open").Role(role.Button)

	// open downloads file.
	if err := uiauto.Combine("open downloads file",
		files.OpenDownloads(),
		files.WithTimeout(10*time.Second).WaitForFile(fileName),
		files.SelectFile(fileName),
		files.WithTimeout(10*time.Second).LeftClick(openButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to open downloads file")
	}

	testing.Sleep(ctx, time.Second)

	if err := files.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close filesapp")
	}

	return nil
}

// selectPrinter here is another way to select printer
func selectPrinter(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	// notice: in ui_tree, items has different name
	// so use NameContaining instead
	// and specify role or classname
	destinationFinder := nodewith.NameContaining("Destination").Role(role.PopUpButton)
	seemoreOptionFinder := nodewith.NameContaining("See more").Role(role.MenuItem)
	printerFinder := nodewith.NameContaining("USB").ClassName("list-item")

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Select a destination",
		ui.LeftClick(destinationFinder),
		ui.LeftClick(seemoreOptionFinder),
		ui.WaitUntilExists(printerFinder),
		ui.LeftClick(printerFinder))(ctx); err != nil {
		return err
	}

	return nil
}

// waitForPrintCompleted Waiting for print job completed
func waitForPrintCompleted(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	// ---- Verify print job status notification is present
	if _, err := ash.WaitForNotification(ctx, tconn, time.Minute, ash.WaitTitleContains("Printing")); err != nil {
		return errors.Wrap(err, "failed to wait for notification")
	}

	// ---- Verify printer produces the printed page successfully(limited to reasonable acceptance)
	s.Log("Waiting for print job to complete")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "lpstat", "-W", "not-completed", "-o").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		if len(out) != 0 {
			return errors.New("Print job has not completed yet")
		}
		testing.ContextLog(ctx, "Print job has completed")
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "print job failed to complete")
	}

	out, err := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o").Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}
	if len(out) == 0 {
		return errors.New("Print job has not completed yet")
	}

	return nil
}

// verifyPrintFile Notice user put file into scanner
// Scan file then copy it to wwcb server (launch scanapp -> select scanner -> press scan -> waitforfile -> copy file to wwcb server)
// Compare wwcb server file and fixture server file
func verifyPrintFile(ctx context.Context, s *testing.State, tconn *chrome.TestConn, key comparePrinterKey) error {

	s.Log("verfiy print file")

	// notice - user put file into scanner
	// msg := "Please put file into scanner"
	// if err := utils.WebNotification(ctx, s, msg); err != nil {
	// 	return err
	// }

	// scan file then copy file to wwcb server
	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch scanapp")
	}
	defer app.Close(ctx)

	startTime := time.Now()

	// press scan
	if err := app.WithTimeout(time.Minute).Scan()(ctx); err != nil {
		return errors.Wrap(err, "failed to click scan")
	}

	// wait file saved
	var pat *regexp.Regexp
	pat = regexp.MustCompile(`^scan_\d{8}-\d{6}[^.]*\.pdf$`)

	// file should be in folder
	fs, err := utils.WaitForFileSaved(ctx, filesapp.MyFilesPath, pat, startTime)
	if err != nil {
		return errors.Wrap(err, "failed to wait for file saved")
	}

	// copy file from chromebook to wwcb server
	filepath := filepath.Join(filesapp.MyFilesPath, fs.Name())

	// upload file to wwcb server
	uploadPath, err := utils.UploadFile(ctx, filepath)
	if err != nil {
		return errors.Wrap(err, "failed to upload file to wwcb server")
	}

	// compare uploaded file and golden sample on wwcb server
	if err := comparePrinterPic(s, key, uploadPath); err != nil {
		return errors.Wrap(err, "failed to compare printer picture")
	}

	return nil
}

// comparePrinterPic compare printer pic
// compare_printer_pic(key:str ,filepath: str)
// key: original, scale, size, quality, headers, background
func comparePrinterPic(s *testing.State, key comparePrinterKey, filepath string) error {

	WWCBServerURL, ok := s.Var("FixtureWebURL")
	if !ok {
		return errors.Errorf("Runtime variable %s is not provided", WWCBServerURL)
	}

	// construct URL
	URL := fmt.Sprintf("%s/api/compare_printer_pic?key=%s&filepath=%s",
		WWCBServerURL,
		string(key),
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
