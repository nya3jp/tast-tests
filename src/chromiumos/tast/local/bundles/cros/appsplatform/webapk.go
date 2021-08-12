// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

type shareResult struct {
	text  string
	title string
	files []string
	err   error
}

const (
	// A pre-generated WebAPK which points to a PWA installed from localhost:8000.
	generatedWebAPK = "WebShareTargetTestWebApk_20210707.apk"

	localServerAddr = "localhost:8000"

	testPackage        = "org.chromium.arc.testapp.chromewebapk"
	testClass          = "org.chromium.arc.testapp.chromewebapk.MainActivity"
	shareTextButtonID  = testPackage + ":id/share_text_button"
	shareFilesButtonID = testPackage + ":id/share_files_button"
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
	// shareChan is a channel containing shared data received through HTTP
	// requests to the test server. Any errors generated asynchronously by
	// the server will also be sent through the channel and will be handled
	// in shareTextAndVerify, after we have triggered an HTTP request to the
	// server.
	server, shareChan := startTestPWAServer(ctx, s.DataFileSystem())
	defer server.Shutdown(cleanupCtx)
	defer close(shareChan)

	appID, err := installTestApps(ctx, cr, a, tconn, s.DataPath(generatedWebAPK))
	if err != nil {
		s.Fatal("Failed to install test apps: ", err)
	}
	defer apps.Close(ctx, tconn, appID)

	activity, err := arc.NewActivity(a, testPackage, testClass)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer activity.Close()
	if err := activity.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the test activity: ", err)
	}
	defer activity.Stop(cleanupCtx, tconn)

	// Click the "Share Text" button and verify that text is received.
	if err := clickShareButton(ctx, a, shareTextButtonID); err != nil {
		s.Fatal("Failed to click share text button in test app: ", err)
	}
	if err := verifySharedText(ctx, shareChan); err != nil {
		s.Fatal("Failed to share text from test app: ", err)
	}

	// Click the "Share Files" button and verify that files are received.
	if err := clickShareButton(ctx, a, shareFilesButtonID); err != nil {
		s.Fatal("Failed to click share files button in test app: ", err)
	}
	if err := verifySharedFiles(ctx, shareChan); err != nil {
		s.Fatal("Failed to share files from test app: ", err)
	}
}

func startTestPWAServer(ctx context.Context, filesystem http.FileSystem) (*http.Server, chan shareResult) {
	shareChan := make(chan shareResult)
	mux := http.NewServeMux()
	fs := http.FileServer(filesystem)
	mux.Handle("/", fs)
	mux.HandleFunc("/share", func(w http.ResponseWriter, r *http.Request) {
		if parseErr := r.ParseMultipartForm(4096); parseErr != nil {
			shareChan <- shareResult{err: errors.Wrap(parseErr, "failed to parse multipart form")}
			return
		}

		var result shareResult

		if len(r.MultipartForm.Value["text"]) == 1 {
			result.text = r.MultipartForm.Value["text"][0]
		}
		if len(r.MultipartForm.Value["title"]) == 1 {
			result.title = r.MultipartForm.Value["title"][0]
		}

		result.files = make([]string, len(r.MultipartForm.File["received_file"]))
		for i, f := range r.MultipartForm.File["received_file"] {
			filecontents, err := f.Open()
			defer filecontents.Close()
			if err != nil {
				shareChan <- shareResult{err: errors.Wrap(err, "failed to open file")}
				return
			}
			bytes, err := ioutil.ReadAll(filecontents)
			if err != nil {
				shareChan <- shareResult{err: errors.Wrap(err, "failed to read file")}
				return
			}
			result.files[i] = string(bytes)
		}

		shareChan <- result
	})

	server := &http.Server{Addr: localServerAddr, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			shareChan <- shareResult{err: errors.Wrap(err, "failed to start local server")}
		}
	}()

	return server, shareChan
}

func installTestApps(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, tconn *chrome.TestConn, webAPKPath string) (string, error) {
	const (
		localServerIndex = "http://" + localServerAddr + "/webshare_index.html"
		installTimeout   = 15 * time.Second
		testAPK          = "ArcChromeWebApkTest.apk"
	)

	appID, err := apps.InstallPWAForURL(ctx, cr, localServerIndex, installTimeout)
	if err != nil {
		return "", errors.Wrap(err, "failed to install PWA for URL")
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, appID, installTimeout); err != nil {
		return "", errors.Wrap(err, "failed to wait for PWA to be installed")
	}
	if err := a.Install(ctx, webAPKPath); err != nil {
		return "", errors.Wrap(err, "failed to install WebAPK")
	}
	if err := a.Install(ctx, arc.APKPath(testAPK)); err != nil {
		return "", errors.Wrap(err, "failed to install test app")
	}
	return appID, nil

}

func clickShareButton(ctx context.Context, a *arc.ARC, buttonID string) error {
	device, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	// Deliberately close the UI Automator server as soon as we're done with
	// it, rather than at the end of the test. On rvc-arc, sharing sometimes
	// does not happen until the UI Automator server is closed.
	defer device.Close(ctx)

	if err := device.WaitForIdle(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for device idle")
	}

	// Clicking the "Share Text" button will send share data directly to
	// any installed WebAPK.
	if err := device.Object(ui.ID(buttonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click share button")
	}
	return nil
}

func verifySharedText(ctx context.Context, shareChan chan shareResult) error {
	const (
		expectedSharedTitle = "Shared title"
		expectedSharedText  = "Shared text"
	)

	var receivedShare shareResult
	select {
	case receivedShare = <-shareChan:
	case <-ctx.Done():
		return errors.New("timeout waiting to receive shared text")
	}

	if receivedShare.err != nil {
		return errors.Wrap(receivedShare.err, "error received from test server")
	}

	if receivedShare.title != expectedSharedTitle {
		return errors.Errorf("failed to match shared title: got %q, want %q", receivedShare.title, expectedSharedTitle)
	}

	if receivedShare.text != expectedSharedText {
		return errors.Errorf("failed to match shared title: got %q, want %q", receivedShare.text, expectedSharedText)
	}
	return nil
}

func verifySharedFiles(ctx context.Context, shareChan chan shareResult) error {
	const (
		expectedFile0 = "{\"text\": \"foobar\"}"
		expectedFile1 = "{\"text\": \"lorem ipsum\"}"
	)

	var receivedShare shareResult
	select {
	case receivedShare = <-shareChan:
	case <-ctx.Done():
		return errors.New("timeout waiting to receive shared text")
	}

	if receivedShare.err != nil {
		return errors.Wrap(receivedShare.err, "error received from test server")
	}

	if receivedShare.files[0] != expectedFile0 {
		return errors.Errorf("failed to match shared file: got %q, want %q", receivedShare.files[0], expectedFile0)
	}
	if receivedShare.files[1] != expectedFile1 {
		return errors.Errorf("failed to match shared file: got %q, want %q", receivedShare.files[1], expectedFile1)
	}

	return nil
}
