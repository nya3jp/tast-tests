// Copyright 2022 The ChromiumOS Authors
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

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
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
		SoftwareDeps: []string{"chrome", "android_vm_r"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Timeout:      5 * time.Minute,
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
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close(ctx)
	defer func() {
		if s.HasError() {
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()
			if err := a.Command(ctx, "uiautomator", "dump").
				Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			} else if err := a.PullFile(ctx, "/sdcard/window_dump.xml",
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
	var targetPath = filepath.Join("/storage/emulated/0/Download/", downloadName)
	var sourceURL = filepath.Join(downloadPath, downloadName)
	defer os.Remove(targetPath)
	defer os.Remove(sourceURL)

	// This is placed here to make sure that, in the event of an error, it is
	// evaluated before the file is deleted, so we get a more useful log and
	// screenshot.
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")

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
	androidPort, err := a.ReverseTCP(ctx, hostPort)
	if err != nil {
		s.Fatal("Failed to start reverse port forwarding: ", err)
	}
	defer a.RemoveReverseTCP(ctx, androidPort)

	dlURL := "http://127.0.0.1:" + strconv.Itoa(androidPort)

	const (
		apkName                        = "ArcDownloadManagerTest.apk"
		packageName                    = "org.chromium.arc.testapp.downloadmanager"
		writeExternalStoragePermission = "android.permission.WRITE_EXTERNAL_STORAGE"
		sourceURLKey                   = "source_url"
		targetPathKey                  = "target_path"
	)

	// Install the test app.
	if err := a.Install(ctx, arc.APKPath(apkName), adb.InstallOptionGrantPermissions); err != nil {
		s.Fatalf("Failed to install %s: %s", apkName, err)
	}

	// Create the MainActivity of the test app.
	act, err := arc.NewActivity(a, packageName, ".MainActivity")
	if err != nil {
		s.Fatalf("Failed to create the main activity for %s: %s", packageName, err)
	}
	if err := act.Start(ctx, tconn,
		arc.WithWaitForLaunch(),
		arc.WithForceStop(),
		arc.WithExtraString(sourceURLKey, dlURL),
		arc.WithExtraString(targetPathKey, targetPath),
	); err != nil {
		act.Close()
		s.Fatalf("Failed to start the main activity for %s: %s", packageName, err)
	}

	if err := uiauto.Combine("check for download chip",
		// Left click the tray to open the bubble.
		uia.LeftClick(holdingspace.FindTray()),
		// Verify that the ARC download exists in holding space.
		// Currently broken due to crbug/1291882
		uia.WaitUntilExists(holdingspace.FindDownloadChip().Name(downloadName+"thisshouldbreak")),
	)(ctx); err != nil {
		s.Fatal("Download chip not found: ", err)
	}
}
