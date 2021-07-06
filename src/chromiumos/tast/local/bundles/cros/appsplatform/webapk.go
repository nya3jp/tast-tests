// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"net/http"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
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
	const (
		testAPK     = "ArcChromeWebApkTest.apk"
		testPackage = "org.chromium.arc.testapp.chromewebapk"
		testClass   = "org.chromium.arc.testapp.chromewebapk.MainActivity"

		localServerAddr  = "localhost:8000"
		localServerIndex = "http://" + localServerAddr + "/webshare_index.html"
		installTimeout   = 15 * time.Second

		shareTextButtonID = testPackage + ":id/share_text_button"

		expectedSharedTitle = "Shared title"
		expectedSharedText  = "Shared text"
	)

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	s.Log("Starting test PWA Server")

	shareChan := make(chan sharedTextAndTitle)
	defer close(shareChan)

	mux := http.NewServeMux()
	fs := http.FileServer(s.DataFileSystem())
	mux.Handle("/", fs)
	mux.HandleFunc("/share", func(w http.ResponseWriter, r *http.Request) {
		s.Log("Handling share HTTP request")
		if parseErr := r.ParseMultipartForm(4096); parseErr != nil {
			s.Fatal("Failed to parse multipart form: ", parseErr)
			return
		}

		sharedText := r.MultipartForm.Value["text"][0]
		sharedTitle := r.MultipartForm.Value["title"][0]

		shareChan <- sharedTextAndTitle{text: sharedText, title: sharedTitle}
	})

	server := &http.Server{Addr: localServerAddr, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local server: ", err)
		}
	}()
	defer server.Shutdown(ctx)

	s.Log("Installing test PWA")

	appID, err := apps.InstallPWAForURL(ctx, cr, localServerIndex, installTimeout)
	if err != nil {
		s.Fatal("Failed to install PWA for URL: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, appID, installTimeout); err != nil {
		s.Fatal("Failed to wait for PWA to be installed: ", err)
	}

	s.Log("Installing test APKs")

	if err := a.Install(ctx, arc.APKPath(testAPK)); err != nil {
		s.Fatal("Failed installing test app: ", err)
	}
	if err := a.Install(ctx, s.DataPath(generatedWebAPK)); err != nil {
		s.Fatal("Failed installing WebAPK: ", err)
	}

	s.Log("Launching test activity")

	act, err := arc.NewActivity(a, testPackage, testClass)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := d.WaitForIdle(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for device idle: ", err)
	}

	s.Log("Sharing from Android to Web")

	// Clicking the "Share Text" button will send share data directly to
	// any installed WebAPK.
	if err := d.Object(ui.ID(shareTextButtonID)).Click(ctx); err != nil {
		s.Fatal("Failed to click share button: ", err)
	}

	var receivedShare sharedTextAndTitle
	select {
	case receivedShare = <-shareChan:
	case <-ctx.Done():
		s.Fatal("Timeout waiting to receive shared text")
	}

	if receivedShare.title != expectedSharedTitle {
		s.Errorf("Shared title did not match: got %q, want %q", receivedShare.title, expectedSharedTitle)
	}

	if receivedShare.text != expectedSharedText {
		s.Errorf("Shared title did not match: got %q, want %q", receivedShare.text, expectedSharedText)
	}
}
