// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcDownload,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that ARC downloads are shown in holding space",
		Contacts: []string{
			"angusmclean@google.com",
			"tote-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func ArcDownload(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.UnRestrictARCCPU(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
	)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Opt in to Play Store.
	maxAttempts := 2
	if err = optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to opt in to Play Store: ", err)
	}
	if err = optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to show Play Store: ", err)
	}

	// Launch ARC and handle error logging.
	arc, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer arc.Close(ctx)
	defer func() {
		if s.HasError() {
			if err := arc.Command(ctx, "uiautomator", "dump").
				Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			} else if err := arc.PullFile(ctx, "/sdcard/window_dump.xml",
				filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	// Ensure the tray does not exist prior to adding anything to holding space.
	uia := uiauto.New(tconn)
	err = uia.EnsureGoneFor(holdingspace.FindTray(), 5*time.Second)(ctx)
	if err != nil {
		s.Fatal("Tray exists: ", err)
	}

	downloadPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get download path: ", err)
	}

	downloadName := "download.txt"
	defer os.Remove(filepath.Join("/storage/emulated/0/Download/", downloadName))
	defer os.Remove(filepath.Join(downloadPath, downloadName))

	// These are placed here to make sure that, in the event of an error, they are
	// evaluated before the file is deleted, so we get a more useful log and
	// screenshot.
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	// Create a local server. Respond to all calls with a text file marked as an
	// attachment to ensure it is downloaded, not shown in browser.
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "text/plain")
			w.Header().Add("Content-Disposition",
				"attachment; filename="+downloadName)
			fmt.Fprintf(w, "Hic sunt leones\n")
		}))
	defer server.Close()

	srvURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to parse test server URL: ", err)
	}

	hostPort, err := strconv.Atoi(srvURL.Port())
	if err != nil {
		s.Fatal("Failed to parse test server port: ", err)
	}

	// By default, the apps inside ARC can't see our test server. `ReverseTCP`
	// forwards the traffic.
	androidPort, err := arc.ReverseTCP(ctx, hostPort)
	if err != nil {
		s.Fatal("Failed to start reverse port forwarding: ", err)
	}
	defer arc.RemoveReverseTCP(ctx, androidPort)

	dlURL := "http://127.0.0.1:" + strconv.Itoa(androidPort)

	// Create a new android UI device so we can interact with ui elements in ARC.
	uid, err := arc.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to create new UI Device: ", err)
	}
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}

	// Install the beta version of Chrome as a way to download a file inside ARC.
	// The production version is not allowed by ARC.
	packageName := "com.chrome.beta"
	if err := playstore.InstallApp(ctx, arc, uid, packageName,
		&playstore.Options{InstallationTimeout: 180 * time.Second}); err != nil {
		s.Fatal("Failed to install chrome from play store: ", err)
	}

	// Open the download URL in Chrome Beta. Sending the specific package prevents
	// ARC from offerring to forward it to ash/lacros chrome.
	if err := arc.Command(ctx, "am", "start", "-a", "android.intent.action.VIEW",
		"-p", packageName, "-d", dlURL).Run(); err != nil {
		s.Fatal("Failed to send intent to open Chrome Beta: ", err)
	}

	defaultUITimeout := 5 * time.Second

	// Get past "Welcome to Chrome" dialogue, if it shows.
	acceptButton := uid.Object(ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)"+"Accept.+"))
	if err := acceptButton.WaitForExists(ctx, defaultUITimeout); err == nil {
		if err := acceptButton.Click(ctx); err != nil {
			s.Fatal("Failed to click accept button: ", err)
		}
	}

	// Click the "No thanks" button on the sync dialogue for the sake of speed.
	noThanksButton := uid.Object(ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)"+"No thanks"))
	if err := noThanksButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Fatal("Failed to find no thanks button: ", err)
	}
	if err := noThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click no thanks button: ", err)
	}

	// Click continue and then allow when prompted to allow access to Chrome OS's
	// file system.
	continueButton := uid.Object(ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)"+"Continue"))
	if err := continueButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Fatal("Failed to find continue button: ", err)
	}
	if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click continue button: ", err)
	}
	allowButton := uid.Object(ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)"+"Allow"))
	if err := allowButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Fatal("Failed to find allow button: ", err)
	}
	if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click allow button: ", err)
	}

	if err := uiauto.Combine("check for download chip",
		// Left click the tray to open the bubble.
		uia.LeftClick(holdingspace.FindTray()),
		// Verify that the ARC download exists in holding space.
		// Currently broken due to crbug/1291882
		uia.WaitUntilExists(holdingspace.FindDownloadChip().Name(downloadName)),
	)(ctx); err != nil {
		s.Fatal("Download chip not found: ", err)
	}
}
