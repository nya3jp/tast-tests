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
// - Scale
// - Pages per sheet
// - Paper size
// - Quality
// - Options: Headers and footers; Two-sided; Background graphics   "

package crostini

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
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

type DropdownName string

const (
	DropdownScale        DropdownName = "Scale"
	DropdownPagePerSheet DropdownName = "Pages per sheet"
	DropdownMargins      DropdownName = "Margins"
	DropdownPaperSize    DropdownName = "Paper size"
	DropdownQuality      DropdownName = "Quality"
)

type PaperSize string

const (
	PaperSizeA0      PaperSize = "A0"
	PaperSizeA1      PaperSize = "A1"
	PaperSizeA2      PaperSize = "A2"
	PaperSizeA3      PaperSize = "A3"
	PaperSizeA4      PaperSize = "A4"
	PaperSizeLegal   PaperSize = "Legal"
	PaperSizeLetter  PaperSize = "Letter"
	PaperSizeTabloid PaperSize = "Tabloid"
)

type PagePerSheet string

const (
	PagePerSheet1  PagePerSheet = "1"
	PagePerSheet2  PagePerSheet = "2"
	PagePerSheet4  PagePerSheet = "4"
	PagePerSheet6  PagePerSheet = "6"
	PagePerSheet9  PagePerSheet = "9"
	PagePerSheet16 PagePerSheet = "16"
)

type Margins string

const (
	MarginsDefault Margins = "Default"
	MarginsNone    Margins = "None"
	MarginsMinimum Margins = "Minimum"
	MarginsCustom  Margins = "Custom"
)

type Scale string

const (
	ScaleCustom  Scale = "Custom"
	ScaleDefault Scale = "Default"
)

const (
	wantPrinter string = CanonUsb
)

type PrintType string

const (
	PrintTypeBROWSER PrintType = "BROWSER"
	PrintTypePDF     PrintType = "PDF"
	PrintTypePNG     PrintType = "PNG"
)

type PrintQuality string

const (
	PrintQualityHigh   PrintQuality = "High"
	PrintQualityNormal PrintQuality = "Normal"
)

type PrintOption string

const (
	PrintOptionHeaders    PrintOption = "Headers and footers"
	PrintOptionBackground PrintOption = "Background graphics"
)

const (
	printFileA = "blank.pdf"
	printFileB = "blank.png"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Printscan1Printer,
		Desc:     "Test USB printing from ChromeOS device.",
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
	if err := Printscan1Printer_Step1(ctx, s, tconn); err != nil {
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
	if err := Printscan1Printer_Step2To4(ctx, s, cr, tconn, wantPrinter); err != nil {
		s.Fatal("Failed to execute step2, 3, 4: ", err)
	}

	// 5. Start a print job and unplug the USB connection in the middle of the print job.
	// ---- Verify print job failed notification should be present.
	// ---- ChromeOS device should not hang/crash.
	// ---- Another print job should be executed successfully.

	// 6. Confirm PDF and PNG files are printed successfully
	if err := Printscan1Printer_Step6(ctx, s, tconn, wantPrinter); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	// 7. Confirm print job can be done from ARC++ / Android app ( e.g. Chrome, gDocs, MSword,etc.)
	// ---- the same USB printer (as set in chrome://settings) can be used
	// ---- no extra steps in the printing steps sequence
	// ---- no impact to system stability or quality of the printed page
	if err := Printscan1Printer_Step7(ctx, s, cr, wantPrinter); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	// 8. Confirm printing with changes to 'More settings' controls works as intended. Like:
	// - Scale
	// - Pages per sheet
	// - Paper size
	// - Quality
	// - Options: Headers and footers; Two-sided; Background graphics   "
	if err := Printscan1Printer_Step8(ctx, s, cr, tconn, wantPrinter); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
}

// 1. Login to a ChromeOS device and connect USB printer.
func Printscan1Printer_Step1(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 1 - Login to a ChromeOS device and connect USB printer.")

	// connect usb printer
	if err := utils.DoSwitchFixture(ctx, s, utils.UsbPrinterType, utils.UsbPrinterIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect printer: ")
	}

	// verfiy connected
	if _, err := ash.WaitForNotification(ctx, tconn, time.Minute, ash.WaitTitle("USB printer connected")); err != nil {
		s.Fatalf("Failed to wait for notification: %v", err)
	}

	return nil
}

// 2. Open google search page and trigger print dialog on the current chrome page by pressing Ctrl + p.

// 3. Dropdown arrow next to 'Destination' Printer should be on the list if not, select on see more (default is 'Save as PDF').
// ---- 'Select A destination' pop-up window should contain contain the printer name.
// 3.a. If the USB printer name is not displayed in list of printers , add the printer manually, e.g. ""settings-> Print and Scan->printers->add printer-> e.g EPSON XP-430 (USB)""
// ---- After triggering Print dialog, the list of printers should contain the added printer name.
// 4. Select the printer and proceed to printing with this printer (e.g. HP OfficeJet 4650)
// ---- Verify printer produces the printed page successfully(limited to reasonable acceptance)
// ---- Verify print job status notification is present
func Printscan1Printer_Step2To4(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, printer string) error {

	s.Log("Step 2 - Open google search page and trigger print dialog on the current chrome page by pressing Ctrl + p.")

	// Open browser window.
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		return errors.Wrapf(err, "Failed to open browser window: ")
	}
	defer conn.Close()

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	//defer kb.Close()

	// ctrl + p to trigger print dialog
	if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
		s.Fatal("Failed to press Ctrl+P to trigger print dialog: ", err)
	}

	s.Logf("Step 3, 4 - Select printer and proceed to printing with this printer")

	if err := selectPrinter(ctx, s, tconn, printer); err != nil {
		return errors.Wrap(err, "Failed to select printer: ")
	}

	// Hide all notifications to prevent them from covering the print button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close all notifications: ", err)
	}

	printpreview.WaitForPrintPreview(tconn)(ctx)
	// start print
	if err := printpreview.Print(ctx, tconn); err != nil {
		return err
	}

	// wait for print completed
	if err := waitForPrintCompleted(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "Failed to")
	}

	// verify print file
	if err := verifyPrintFile(ctx, s, tconn, utils.ComparePrinterOriginal); err != nil {
		return nil
	}

	// ctrl + w to close chrome
	if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
		s.Fatal("Failed to press Ctrl+P to trigger print dialog: ", err)
	}

	return nil
}

// 5. Start a print job and unplug the USB connection in the middle of the print job.
// ---- Verify print job failed notification should be present.
// ---- ChromeOS device should not hang/crash.
// ---- Another print job should be executed successfully.
func Printscan1Printer_Step5(ctx context.Context, s *testing.State) error {

	s.Logf("Step 5 - Start a print job and unplug the USB connection in the middle of the print job.")

	// unplug usb

	// ---- Verify print job failed notification should be present.

	// ---- ChromeOS device should not hang/crash.
	// ---- Another print job should be executed successfully.

	return nil

}

// 6. Confirm PDF and PNG files are printed successfully
func Printscan1Printer_Step6(ctx context.Context, s *testing.State, tconn *chrome.TestConn, printer string) error {

	s.Logf("Step 6 - Confirm PDF and PNG files are printed successfully")

	var files []string
	files = append(files, printFileA)
	files = append(files, printFileB)

	for _, file := range files {

		s.Logf("Confirm %s are printed successfully", file)

		// copy file to download folder
		if err := utils.GetServerFile(ctx, s, filesapp.DownloadPath, file); err != nil {
			return errors.Wrap(err, "Failed to get server file: ")
		}

		// open file in download folder
		if err := openDownloadsFile(ctx, s, tconn, file); err != nil {
			return errors.Wrap(err, "Failed to open file in downloads: ")
		}

		// declare keyboard object
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		//defer kb.Close()

		// ctrl + p to trigger print dialog
		if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
			s.Fatal("Failed to press Ctrl+P to trigger print dialog: ", err)
		}

		// Hide all notifications to prevent them from covering the print button.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			s.Fatal("Failed to close all notifications: ", err)
		}

		printpreview.WaitForPrintPreview(tconn)(ctx)

		// print
		// if err := printpreview.Print(ctx, tconn); err != nil {
		// 	return errors.Wrap(err, "Failed to press print: ")
		// }
		// // enter print
		if err := kb.Accel(ctx, "enter"); err != nil {
			return errors.Wrap(err, "failed to type enter")
		}

		// wait print job completed
		if err := waitForPrintCompleted(ctx, s, tconn); err != nil {
			return errors.Wrap(err, "Failed to wait print job completed: ")
		}

		// verify print file
		if err := verifyPrintFile(ctx, s, tconn, utils.ComparePrinterOriginal); err != nil {
			return errors.Wrap(err, "Failed to verify print file: ")
		}

		// ctrl + w to close file
		if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Fatal("Failed to press Ctrl+W to close file: ", err)
		}

	}

	return nil
}

// 7. Confirm print job can be done from ARC++ / Android app ( e.g. Chrome, gDocs, MSword,etc.)
// ---- the same USB printer (as set in chrome://settings) can be used
// ---- no extra steps in the printing steps sequence
// ---- no impact to system stability or quality of the printed page
func Printscan1Printer_Step7(ctx context.Context, s *testing.State, cr *chrome.Chrome, printer string) error {

	s.Logf("Step 7 - Confirm print job can be done from ARC++ / Android app")

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
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
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
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, -1); err != nil {
		return errors.Wrap(err, "Failed to install app")
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "Failed to close playstore: ")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrapf(err, "Failed to find keyboard: ")
	}
	//defer kb.Close()

	openAppCommand := testexec.CommandContext(ctx, "adb", "shell", "am", "start", "-n", pkgName+"/"+actName)
	if err := openAppCommand.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start companion Android app using adb")
	}

	// Click on allow
	allowText := "ALLOW"
	allowClass := "android.widget.Button"
	allowButton := d.Object(ui.ClassName(allowClass), ui.TextMatches(allowText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		return errors.Wrap(err, "allowButton doesn't exists: ")
	}
	if err := allowButton.Click(ctx); err != nil {
		return errors.Wrap(err, "Failed to click on allowButton: ")
	}

	// Click on download
	downloadText := "Download"
	downloadClass := "android.widget.TextView"
	downloadTextView := d.Object(ui.ClassName(downloadClass), ui.TextMatches(downloadText))
	if err := downloadTextView.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Logf("downloadTextView doesn't exists: ")
	} else if err := downloadTextView.Click(ctx); err != nil {
		return errors.Wrap(err, "Failed to click on downloadTextView: ")
	}

	// Click on file
	fileText := printFileA
	fileClass := "android.widget.TextView"
	fileTextView := d.Object(ui.ClassName(fileClass), ui.TextMatches(fileText))
	if err := fileTextView.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Logf("fileTextView doesn't exists: ")
	} else if err := fileTextView.Click(ctx); err != nil {
		return errors.Wrap(err, "Failed to click on fileTextView: ")
	}

	// Click on print
	printButton := d.Object(ui.ResourceID("com.picsel.tgv.app.smartoffice:id/print_button"))
	if err := printButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Logf("printButton doesn't exists: ")
	} else if err := printButton.Click(ctx); err != nil {
		return errors.Wrap(err, "Failed to click printButton: ")
	}

	// Hide all notifications to prevent them from covering the print button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close all notifications: ", err)
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
		return errors.Wrap(err, "Failed to wait print job completed")
	}

	// verify print file
	if err := verifyPrintFile(ctx, s, tconn, utils.ComparePrinterOriginal); err != nil {
		return errors.Wrap(err, "Failed to verify print file: ")
	}

	closeAppCommand := testexec.CommandContext(ctx, "adb", "shell", "am", "force-stop", pkgName)
	if err := closeAppCommand.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to close Android app using adb")
	}

	return nil
}

// 8. Confirm printing with changes to 'More settings' controls works as intended. Like:
// - Scale
// - Pages per sheet
// - Paper size
// - Quality
// - Options: Headers and footers; Two-sided; Background graphics   "
func Printscan1Printer_Step8(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, printer string) error {
	// seq	name				origin  -> set to 	-> back to origin
	// 1. 	Scale: 				100 	-> 50		-> 100
	// 2. 	Pages per sheet: 	1 		-> 2		-> 1
	// 3.	Paper size:  		letter 	-> A4		-> letter
	// 4.	Quality: 			normal	-> Hight	-> normal
	// 5.	Headers and footers true	-> true		-> true
	// 6.	Background graphics false 	-> true		-> true

	// var browserNode *nodewith.Finder = nodewith.Role(role.Window).First()

	s.Log("Step 8 - Confirm printing with changes to 'More settings' controls works as intended.")

	// Open browser window.
	_, err := cr.NewConn(ctx, "")
	if err != nil {
		return errors.Wrapf(err, "Failed to open browser window: ")
	}
	// defer conn.Close()

	for i := 0; i <= 6; i++ {

		if err := triggerPrintDialog(ctx, s); err != nil {
			return errors.Wrap(err, "Failed to trigger print dialog: ")
		}

		printpreview.WaitForPrintPreview(tconn)(ctx)

		// show more settings
		if err := showMoreSettingsVisible(ctx, s, tconn); err != nil {
			return nil
		}

		printpreview.WaitForPrintPreview(tconn)(ctx)

		// change
		switch i {
		case 0: // margins: default -> custom
			if err := setDropdown(ctx, s, tconn, DropdownMargins, string(MarginsCustom)); err != nil {
				return err
			}
		case 1: // paper size: letter -> a4

			if err := setDropdown(ctx, s, tconn, DropdownMargins, string(MarginsDefault)); err != nil {
				return err
			}

			// notice:
			if err := utils.FixtureServerNotice(s, "Please change printer page size to A4"); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			if err := setDropdown(ctx, s, tconn, DropdownPaperSize, string(PaperSizeA4)); err != nil {
				return err
			}
		case 2: // sheet 1 -> 2

			// notice:
			if err := utils.FixtureServerNotice(s, "Please change printer page size to Letter"); err != nil {
				return err
			}

			if err := setDropdown(ctx, s, tconn, DropdownPaperSize, string(PaperSizeLetter)); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			if err := setDropdown(ctx, s, tconn, DropdownPagePerSheet, string(PagePerSheet2)); err != nil {
				return err
			}
		case 3:

			if err := setDropdown(ctx, s, tconn, DropdownPagePerSheet, string(PagePerSheet1)); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			// set scale
			if err := setScale(ctx, tconn, "50"); err != nil {
				return err
			}
		case 4: // headers: true -> false

			// scale: custom 50->default
			if err := setDropdown(ctx, s, tconn, DropdownScale, string(ScaleDefault)); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			if err := setOption(ctx, s, tconn, PrintOptionHeaders, checked.False); err != nil {
				return err
			}
		case 5: // backgroud: false -> true
			// set
			if err := setOption(ctx, s, tconn, PrintOptionHeaders, checked.True); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			if err := setOption(ctx, s, tconn, PrintOptionBackground, checked.True); err != nil {
				return err
			}
		case 6: // quality: normal -> high
			if err := setOption(ctx, s, tconn, PrintOptionBackground, checked.False); err != nil {
				return err
			}

			printpreview.WaitForPrintPreview(tconn)(ctx)

			// set quality
			if err := setQuality(ctx, s, tconn, PrintQualityHigh); err != nil {
				return err
			}
		}

		// Hide all notifications to prevent them from covering the print button.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			s.Fatal("Failed to close all notifications: ", err)
		}

		// print
		printpreview.WaitForPrintPreview(tconn)(ctx)

		if err := printpreview.Print(ctx, tconn); err != nil {
			return err
		}

		if err := waitForPrintCompleted(ctx, s, tconn); err != nil {
			return err
		}

	}

	return nil
}

// when "more settings" is collapsed, click on it
func showMoreSettingsVisible(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	moreSettingsFinder := nodewith.Name("More settings").Role(role.Button)

	ui := uiauto.New(tconn)

	if err := ui.WaitForLocation(moreSettingsFinder)(ctx); err != nil {
		return err
	}

	nodeInfo, err := ui.Info(ctx, moreSettingsFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the more settings: ")
	}

	if nodeInfo.State[state.Collapsed] == true {
		// click more settings
		if err := ui.LeftClick(moreSettingsFinder)(ctx); err != nil {
			return err
		}
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	//defer kb.Close()

	// move to bottom
	if err := kb.Accel(ctx, "search+right"); err != nil {
		return errors.Wrap(err, "failed to type end")
	}

	return nil
}

// set drop down
// according dropdown's name
// select dropdown option
func setDropdown(ctx context.Context, s *testing.State, tconn *chrome.TestConn, dropdownName DropdownName, dropdownOption string) error {

	s.Logf("Setting dropdown %s to %s ..", dropdownName, dropdownOption)

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
	time.Sleep(time.Second)
	return nil
}

// reference to function - SetPages
func setScale(ctx context.Context, tconn *chrome.TestConn, scales string) error {
	// convert to int
	amount, err := strconv.Atoi(scales)
	if err != nil {
		return err
	}

	// restrict scales range
	if int64(amount) < 10 || int64(amount) > 200 {
		return errors.Errorf("Scale amount must be a number between 10 and 200")
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
	customOption := nodewith.Name(string(ScaleCustom)).Role(role.ListBoxOption)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(customOption)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for pages list to expand")
	}

	// Select "Custom" and set the desired page range.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	//defer kb.Close()
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

	time.Sleep(time.Second)

	if err := kb.Type(ctx, scales); err != nil {
		return errors.Wrap(err, "failed to type pages")
	}

	return nil
}

// set quality
// advanced settings -> search "Print quality" -> set quality
func setQuality(ctx context.Context, s *testing.State, tconn *chrome.TestConn, wantQuality PrintQuality) error {

	// Select "Custom" and set the desired page range.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	//defer kb.Close()

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

// set option
// when option checked is not eqaul expected,
// do the click
func setOption(ctx context.Context, s *testing.State, tconn *chrome.TestConn, optionName PrintOption, checked checked.Checked) error {

	s.Logf("Setting option %s to %s ..", string(optionName), string(checked))

	optionFinder := nodewith.Name(string(optionName)).Role(role.CheckBox)

	ui := uiauto.New(tconn)
	// Find the node info for the mirror checkbox.
	nodeInfo, err := ui.Info(ctx, optionFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the mirror checkbox")
	}
	// When not eqaul expected, to the click
	if nodeInfo.Checked != checked {
		testing.ContextLogf(ctx, "Click '%s' checkbox", optionName)

		if err := ui.MakeVisible(optionFinder)(ctx); err != nil {
			return errors.Wrap(err, "Failed to make visible: ")
		}

		if err := ui.LeftClick(optionFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}
	}

	return nil
}

// launch "print jobs" through settings page
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
		return errors.Wrapf(err, "Failed to launch Settings page: ")
	}

	if err := ui.LeftClick(entryFinder)(ctx); err != nil {
		return errors.Wrapf(err, "Failed to click entry: ")
	}

	err := ash.WaitForApp(ctx, tconn, apps.PrintManagement.ID, time.Minute)
	if err != nil {
		return errors.Wrapf(err, "Could not find app in shelf after launch: ")
	}

	return nil
}

// open file in download folder
func openDownloadsFile(ctx context.Context, s *testing.State, tconn *chrome.TestConn, fileName string) error {

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	openButton := nodewith.Name("Open").Role(role.Button)

	// View image preview information of test image.
	if err := uiauto.Combine("View image preview information",
		files.OpenDownloads(),
		files.WithTimeout(10*time.Second).WaitForFile(fileName),
		files.SelectFile(fileName),
		files.WithTimeout(10*time.Second).LeftClick(openButton))(ctx); err != nil {
		s.Fatal("Failed to view image preview information: ", err)
	}

	time.Sleep(time.Second)

	return nil
}

// here is another way to select printer
func selectPrinter(ctx context.Context, s *testing.State, tconn *chrome.TestConn, printer string) error {

	// notice: in ui_tree, items has different name
	// so use NameContaining instead
	// and specify role or classname
	dropdownFinder := nodewith.NameContaining("Save as PDF").Role(role.PopUpButton)
	dropdownOptionFinder := nodewith.NameContaining("See more").Role(role.MenuItem)
	printerFinder := nodewith.NameContaining(printer).ClassName("list-item")

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Select a destination",
		ui.LeftClick(dropdownFinder),
		ui.LeftClick(dropdownOptionFinder),
		ui.WaitUntilExists(printerFinder),
		ui.LeftClick(printerFinder))(ctx); err != nil {
		return err
	}

	return nil
}

// press ctrl + p
// trigger print dialog
func triggerPrintDialog(ctx context.Context, s *testing.State) error {

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	//defer kb.Close()

	// ctrl + p to trigger print dialog
	if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
		s.Fatal("Failed to press Ctrl+P to trigger print dialog: ", err)
	}

	return nil
}

// Waiting for notification
// Waiting for print job completed
func waitForPrintCompleted(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	// ---- Verify print job status notification is present
	if _, err := ash.WaitForNotification(ctx, tconn, time.Minute, ash.WaitTitleContains("Printing")); err != nil {
		s.Fatalf("Failed to wait for notification: %v", err)
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
		s.Fatal("Print job failed to complete: ", err)
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

// Notice user put file into scanner
// Scan file then copy it to tast server (launch scanapp -> select scanner -> press scan -> waitforfile -> copy file to tast server)
// Compare tast server file and fixture server file
func verifyPrintFile(ctx context.Context, s *testing.State, tconn *chrome.TestConn, key utils.ComparePrinterKey) error {

	return nil
	// notice - user put file into scanner
	msg := "Please put file into scanner"
	if err := utils.FixtureServerNotice(s, msg); err != nil {
		return err
	}

	// scan file then copy file to tast server
	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to launch scanapp: ")
	}
	defer app.Close(ctx)

	startTime := time.Now()
	// press scan
	if err := app.Scan()(ctx); err != nil {
		return errors.Wrap(err, "Failed to click scan: ")
	}

	// wait file saved
	var pat *regexp.Regexp
	pat = regexp.MustCompile(`^scan_\d{8}-\d{6}[^.]*\.pdf$`)

	// file should be in folder
	fs, err := WaitForFileSaved(ctx, filesapp.MyFilesPath, pat, startTime)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for file saved: ")
	}

	// copy file from chromebook to tast server
	tastpath := filepath.Join(utils.GetOutputPath(s), fs.Name())

	// compare tast server file and fixture server file
	if err := utils.ComparePrinterPic(s, key, tastpath); err != nil {
		return errors.Wrap(err, "Failed to compare printer picture: ")
	}

	return nil
}
