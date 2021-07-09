// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"net/http"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

type sharedTextAndTitle struct {
	text  string
	title string
}

const (
	// A pre-generated WebAPK which points to a PWA installed from localhost:8000.
	generatedWebAPK = "WebShareTargetTestWebApk_20210707.apk"

	localServerAddr = "localhost:8000"

	testPackage = "org.chromium.arc.testapp.chromewebapk"
	testClass   = "org.chromium.arc.testapp.chromewebapk.MainActivity"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebAPK,
		Desc: "Checks that a WebAPK can be used to share data to a web app",
		Contacts: []string{
			"tsergeant@chromium.org",
			"jinrongwu@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "arcBootedWithWebAppSharing",
		Data: []string{
			"webshare_icon.png",
			"webshare_index.html",
			"webshare_manifest.json",
			"webshare_service.js",
			generatedWebAPK,
		},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// WebAPK verifies that sharing to a WebAPK launches the corresponding Web App
// with the shared data attached.
func WebAPK(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	// Reserve time for cleanup operations.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	s.Log("Starting test PWA Server")
	server, shareChan := startTestPWAServer(ctx, s)
	defer server.Shutdown(cleanupCtx)
	defer close(shareChan)

	s.Log("Installing test apps")
	installTestApps(ctx, s, cr, a, tconn)

	s.Log("Launching test app")
	device, activity := launchTestApp(ctx, s, a, tconn)
	defer activity.Close()
	defer activity.Stop(cleanupCtx, tconn)
	defer device.Close(cleanupCtx)

	s.Log("Sharing from Android to Web")
	shareTextAndVerify(ctx, s, device, shareChan)
}

func startTestPWAServer(ctx context.Context, s *testing.State) (*http.Server, chan sharedTextAndTitle) {
	shareChan := make(chan sharedTextAndTitle)
	mux := http.NewServeMux()
	fs := http.FileServer(s.DataFileSystem())
	mux.Handle("/", fs)
	mux.HandleFunc("/share", func(w http.ResponseWriter, r *http.Request) {
		s.Log("Handling share HTTP request")
		if parseErr := r.ParseMultipartForm(4096); parseErr != nil {
			s.Fatal("Failed to parse multipart form: ", parseErr)
		}

		sharedText := r.MultipartForm.Value["text"]
		if len(sharedText) != 1 {
			s.Fatal("Did not receive shared text")
		}
		sharedTitle := r.MultipartForm.Value["title"]
		if len(sharedTitle) != 1 {
			s.Fatal("Did not receive shared title")
		}

		shareChan <- sharedTextAndTitle{text: sharedText[0], title: sharedTitle[0]}
	})

	server := &http.Server{Addr: localServerAddr, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local server: ", err)
		}
	}()

	return server, shareChan
}

func installTestApps(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, tconn *chrome.TestConn) {
	const (
		localServerIndex = "http://" + localServerAddr + "/webshare_index.html"
		installTimeout   = 15 * time.Second
		testAPK          = "ArcChromeWebApkTest.apk"
	)

	appID, err := apps.InstallPWAForURL(ctx, cr, localServerIndex, installTimeout)
	if err != nil {
		s.Fatal("Failed to install PWA for URL: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, appID, installTimeout); err != nil {
		s.Fatal("Failed to wait for PWA to be installed: ", err)
	}
	if err := a.Install(ctx, s.DataPath(generatedWebAPK)); err != nil {
		s.Fatal("Failed installing WebAPK: ", err)
	}
	if err := a.Install(ctx, arc.APKPath(testAPK)); err != nil {
		s.Fatal("Failed installing test app: ", err)
	}

}

func launchTestApp(ctx context.Context, s *testing.State, a *arc.ARC, tconn *chrome.TestConn) (*ui.Device, *arc.Activity) {
	activity, err := arc.NewActivity(a, testPackage, testClass)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	if err := activity.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}

	device, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}

	if err := device.WaitForIdle(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for device idle: ", err)
	}

	return device, activity
}

func shareTextAndVerify(ctx context.Context, s *testing.State, device *ui.Device, shareChan chan sharedTextAndTitle) {
	const (
		shareTextButtonID = testPackage + ":id/share_text_button"

		expectedSharedTitle = "Shared title"
		expectedSharedText  = "Shared text"
	)

	// Clicking the "Share Text" button will send share data directly to
	// any installed WebAPK.
	if err := device.Object(ui.ID(shareTextButtonID)).Click(ctx); err != nil {
		s.Fatal("Failed to click share button: ", err)
	}

	var receivedShare sharedTextAndTitle
	select {
	case receivedShare = <-shareChan:
	case <-ctx.Done():
		s.Fatal("Timeout waiting to receive shared text")
	}

	if receivedShare.title != expectedSharedTitle {
		s.Fatalf("Shared title did not match: got %q, want %q", receivedShare.title, expectedSharedTitle)
	}

	if receivedShare.text != expectedSharedText {
		s.Fatalf("Shared title did not match: got %q, want %q", receivedShare.text, expectedSharedText)
	}
}
