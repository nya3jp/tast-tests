package utils

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func installDocsFromPlaystore(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, pkgName, appName string) error {

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
		s.Fatal("Failed to install app: ", err)
	}

	// create keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// launch scan via launcher
	if err := launcher.SearchAndLaunch(tconn, kb, appName)(ctx); err != nil {
		return err
	}

	// Click on doc file
	docName := "Untitled document"
	docLabelId := "com.google.android.apps.docs.editors.docs:id/entry_label"
	docLabel := d.Object(ui.ID(docLabelId), ui.TextMatches(docName))
	if err := docLabel.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("docLabel doesn't exists: ", err)
	} else if err := docLabel.Click(ctx); err != nil {
		s.Fatal("Failed to click on docLabel: ", err)
	}

	// custom button
	customButtonId := "com.google.android.apps.docs.editors.docs:id/custom_overflow"
	customButton := d.Object(ui.ID(customButtonId))
	if err := customButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := customButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Share & export
	shareAndrExportText := "Share & export"
	shareAndrExportTextView := d.Object(ui.Text(shareAndrExportText))
	if err := shareAndrExportTextView.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("shareAndrExportTextView doesn't exists: ", err)
	} else if err := shareAndrExportTextView.Click(ctx); err != nil {
		s.Fatal("Failed to click on shareAndrExportTextView: ", err)
	}

	// Print
	printText := "Print"
	printTextView := d.Object(ui.Text(printText))
	if err := printTextView.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("printTextView doesn't exists: ", err)
	} else if err := printTextView.Click(ctx); err != nil {
		s.Fatal("Failed to click on printTextView: ", err)
	}

	return nil
}

func copyUsbFile(ctx context.Context, s *testing.State, filename string) error {

	// through usb

	// to get usb path, sth like /media/removable/{$usbName}
	getUsbPath := testexec.CommandContext(ctx, "sh", "-c", "sudo lsblk -l -o mountpoint | grep removable")
	usbPath, err := getUsbPath.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(getUsbPath.Args))
	}

	// copy file from usb to "Downloads" folder
	copyFile := testexec.CommandContext(ctx, "cp",
		filepath.Join(strings.TrimSpace(string(usbPath)), filename),
		filesapp.DownloadPath)

	if err = copyFile.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(copyFile.Args))
	}

	return nil
}

func copyFileToTast(ctx context.Context, s *testing.State, chromebookPath string) error {
	// retrieve filename
	_, filename := filepath.Split(chromebookPath)

	// transfer file to tast env
	dir, ok := testing.ContextOutDir(ctx)
	if ok && dir != "" {
		if _, err := os.Stat(dir); err == nil {
			testing.ContextLogf(ctx, "copy file to %s", dir)

			// read file
			b, err := ioutil.ReadFile(chromebookPath)
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

	return nil
}
