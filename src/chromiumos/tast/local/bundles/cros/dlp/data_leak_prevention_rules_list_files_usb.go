// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/restrictionlevel"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type fileUsbCopyTestParams struct {
	restriction restrictionlevel.RestrictionLevel
	browserType browser.Type
}

// filesUsbCopyBlockPolicy returns a DLP policy that blocks copying file to USB.
func filesUsbCopyBlockPolicy() []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copying of confidential file to USB",
				Description: "User should not be able to copy confidential file to USB drive",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						"*",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListValueDestinations{
					Components: []string{
						"USB",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "FILES",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}
}

// filesUsbCopyWarnPolicy returns a DLP policy that warns when copying file to USB.
func filesUsbCopyWarnPolicy() []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Warn before copying of confidential file to USB",
				Description: "User should be warened before copy confidential file to USB drive",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						"*",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListValueDestinations{
					Components: []string{
						"USB",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "FILES",
						Level: "WARN",
					},
				},
			},
		},
	},
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListFilesUsb,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with file copy to USB restriction",
		Timeout:      20 * time.Minute,
		Contacts: []string{
			"poromov@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Params: []testing.Param{
			{
				Name:    "ash_allowed",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeAsh,
					restriction: restrictionlevel.Allowed,
				},
			}, {
				Name:              "lacros_allowed",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeLacros,
					restriction: restrictionlevel.Allowed,
				},
			}, {
				Name:    "ash_blocked",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeAsh,
					restriction: restrictionlevel.Blocked,
				},
			}, {
				Name:              "lacros_blocked",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeLacros,
					restriction: restrictionlevel.Blocked,
				},
			}, {
				Name:    "ash_warn_proceeded",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeAsh,
					restriction: restrictionlevel.WarnProceeded,
				},
			}, {
				Name:              "lacros_warn_proceeded",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeLacros,
					restriction: restrictionlevel.WarnProceeded,
				},
			}, {
				Name:    "ash_warn_cancelled",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeAsh,
					restriction: restrictionlevel.WarnCancelled,
				},
			}, {
				Name:              "lacros_warn_cancelled",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: fileUsbCopyTestParams{
					browserType: browser.TypeLacros,
					restriction: restrictionlevel.WarnCancelled,
				},
			},
		},
		Data: []string{
			"download.html",
			"data.txt",
		},
	})
}

func DataLeakPreventionRulesListFilesUsb(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// Setup test HTTP server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Update the policy blob.
	pb := policy.NewBlob()
	if s.Param().(fileUsbCopyTestParams).restriction == restrictionlevel.Blocked {
		pb.AddPolicies(filesUsbCopyBlockPolicy())
	} else if s.Param().(fileUsbCopyTestParams).restriction == restrictionlevel.WarnCancelled || s.Param().(fileUsbCopyTestParams).restriction == restrictionlevel.WarnProceeded {
		pb.AddPolicies(filesUsbCopyWarnPolicy())
	}

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// Clear Downloads directory.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	files, err := ioutil.ReadDir(downloadsPath)
	if err != nil {
		s.Fatal("Failed to get files from Downloads directory")
	}
	for _, file := range files {
		if err = os.RemoveAll(filepath.Join(downloadsPath, file.Name())); err != nil {
			s.Fatal("Failed to remove file: ", file.Name())
		}
	}

	tconnAsh, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	// Ensure that there are no windows open.
	if err := ash.CloseAllWindows(ctx, tconnAsh); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}
	// Ensure that all windows are closed after test.
	defer ash.CloseAllWindows(cleanupCtx, tconnAsh)

	// Create Browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(fileUsbCopyTestParams).browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	tconnBrowser, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to browser's test API: ", err)
	}

	// The browsers sometimes restore some tabs, so we manually close all unneeded tabs.
	closeTabsFunc := browser.CloseAllTabs
	if s.Param().(fileUsbCopyTestParams).browserType == browser.TypeLacros {
		// For lacros-Chrome, it should leave a new tab to keep the Chrome process alive.
		closeTabsFunc = browser.ReplaceAllTabsWithSingleNewTab
	}
	if err := closeTabsFunc(ctx, tconnBrowser); err != nil {
		s.Fatal("Failed to close all unneeded tabs: ", err)
	}
	defer closeTabsFunc(cleanupCtx, tconnBrowser)

	// Open the local page with the file to download.
	conn, err := br.NewConn(ctx, server.URL+"/download.html")
	if err != nil {
		s.Fatal("Failed to open browser: ", err)
	}
	defer conn.Close()

	// Close all prior notifications.
	if err := ash.CloseNotifications(ctx, tconnAsh); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	// The file name is also the ID of the link elements, download it.
	if err := conn.Eval(ctx, `document.getElementById('data.txt').click()`, nil); err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}

	dlFileName := "data.txt"

	// Wait for the notification about downloaded file.
	ntfctn, err := ash.WaitForNotification(
		ctx,
		tconnAsh,
		3*time.Minute,
		ash.WaitIDContains("notification-ui-manager"),
		ash.WaitMessageContains(dlFileName),
	)
	if err != nil {
		s.Fatalf("Failed to wait for notification with title %q: %v", "", err)
	}
	if ntfctn.Title != "Download complete" {
		s.Fatal("Download should be allowed, but wasn't. Notification: ", ntfctn)
	}

	// Create the virtual USB device
	if err := setupVirtualUsbDevice(ctx); err != nil {
		s.Fatal("Fail to setup virtual USB device: ", err)
	}
	defer cleanupVirtualUsbDevice(ctx)

	// Open the Files app, prepare USB drive and try to copy the file.
	filesApp, err := filesapp.Launch(ctx, tconnAsh)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	if err := filesApp.OpenUsbDrive()(ctx); err != nil {
		s.Fatal("Failed to open unformatted USB drive: ", err)
	}
	if err := filesApp.FormatDevice()(ctx); err != nil {
		s.Fatal("Failed to format USB drive: ", err)
	}
	if err := filesApp.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder: ", err)
	}
	if err := filesApp.SelectFile(dlFileName)(ctx); err != nil {
		s.Fatal("Failed to select downloaded file: ", err)
	}
	if err := filesApp.CopyFileToClipboard(dlFileName)(ctx); err != nil {
		s.Fatal("Failed to copy downloaded file to the clipboard: ", err)
	}
	if err := filesApp.OpenUsbDriveWithName("UNTITLED")(ctx); err != nil {
		s.Fatal("Failed to open formatted USB drive: ", err)
	}
	if err := filesApp.PasteFileFromClipboard(keyboard)(ctx); err != nil {
		s.Fatal("Failed to paste copied file: ", err)
	}

	if s.Param().(fileUsbCopyTestParams).restriction == restrictionlevel.WarnProceeded {
		// Hit Enter, which is equivalent to clicking on the "Proceed" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			s.Fatal("Failed to hit Enter: ", err)
		}
	}

	if s.Param().(fileUsbCopyTestParams).restriction == restrictionlevel.WarnCancelled {
		// Hit Esc, which is equivalent to clicking on the "Cancel" button.
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			s.Fatal("Failed to hit Esc: ", err)
		}
	}

	if s.Param().(fileUsbCopyTestParams).restriction == restrictionlevel.WarnCancelled || s.Param().(fileUsbCopyTestParams).restriction == restrictionlevel.Blocked {
		if err := filesApp.EnsureFileGone(dlFileName, 10*time.Second)(ctx); err != nil {
			s.Error("File was copied while it shouldn't: ", err)
		}
	} else {
		if err := filesApp.WaitForFile(dlFileName)(ctx); err != nil {
			s.Error("File was not copied while it should: ", err)
		}
	}

}

// Constants to create a virtual USB drive.
const (
	usbVID          = "dddd"
	usbPID          = "ffff"
	usbManufacturer = "Tast"
	usbProduct      = "VirtualTestUSBDrive"
	usbSerialNumber = "12345"
)

// setupVirtualUsbDevice creates a virtual USB drive.
func setupVirtualUsbDevice(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "modprobe",
		"dummy_hcd").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "Fail to load dummy_hcd module")
	}

	if err := testexec.CommandContext(ctx, "dd", "bs=1024", "count=64", "if=/dev/zero",
		"of=/tmp/backing_file").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "Fail to create temporary backing_file")
	}

	if err := testexec.CommandContext(ctx, "modprobe", "g_mass_storage",
		"file=/tmp/backing_file", "idVendor=0x"+usbVID, "idProduct=0x"+usbPID,
		"iManufacturer="+usbManufacturer, "iProduct="+usbProduct,
		"iSerialNumber="+usbSerialNumber).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "Fail to create virtual USB storage")
	}

	return nil
}

// cleanupVirtualUsbDevice removes previously create virtual USB drive.
func cleanupVirtualUsbDevice(ctx context.Context) {
	testexec.CommandContext(ctx, "modprobe", "g_mass_storage", "-r").Run()
}
