// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/appsplatform/webapks"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/webapk"
	"chromiumos/tast/testing"
)

type shareResult struct {
	text  string
	title string
	files []string
	err   error
}

const (
	testPackage        = "org.chromium.arc.testapp.chromewebapk"
	testClass          = "org.chromium.arc.testapp.chromewebapk.MainActivity"
	shareTextButtonID  = testPackage + ":id/share_text_button"
	shareFilesButtonID = testPackage + ":id/share_files_button"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebAPK,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that a WebAPK can be used to share data to a web app",
		Contacts: []string{
			"tsergeant@chromium.org",
			"jinrongwu@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data: []string{
			"webshare_icon.png",
			"webshare_manifest.json",
			"webshare_service.js",
			webapks.WebShareTargetWebApk.ApkDataPath,
			webapks.WebShareTargetWebApk.IndexPageDataPath,
		},
		Params: []testing.Param{{
			Val:               browser.TypeAsh,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			Val:               browser.TypeAsh,
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
		}, {
			Name:              "lacros_vm",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 3*time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

// WebAPK verifies that sharing to a WebAPK launches the corresponding Web App
// with the shared data attached.
func WebAPK(ctx context.Context, s *testing.State) {
	// Reserve time for cleanup operations.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Due to the UI Automator flakiness, we still can't use the arcBooted fixture as it starts UI Automator automatically.
	var opts []chrome.Option
	opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCEnabled())
	if s.Param().(browser.Type) == browser.TypeLacros {
		lacrosOpts, err := lacrosfixt.NewConfig().Opts()
		if err != nil {
			s.Fatal("Failed to get Lacros options: ", err)
		}
		opts = append(opts, lacrosOpts...)
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	wm, err := webapk.NewManager(ctx, cr, br, a, s, webapks.WebShareTargetWebApk)
	if err != nil {
		s.Fatal("Failed to create WebAPK Manager: ", err)
	}

	defer func(ctx context.Context) {
		if err := wm.UninstallPwa(ctx); err != nil {
			s.Log("Failed to uninstall the test app, it might cause failure in the future run: ", err)
		}
	}(cleanupCtx)

	s.Log("Starting test PWA Server")
	// shareChan is a channel containing shared data received through HTTP
	// requests to the test server. Any errors generated asynchronously by
	// the server will also be sent through the channel and will be handled
	// in shareTextAndVerify, after we have triggered an HTTP request to the
	// server.
	shareChan := startTestPWAServer(ctx, wm, s.DataFileSystem())
	defer wm.ShutdownServer(cleanupCtx)
	defer close(shareChan)

	if err = installTestApps(ctx, wm, a); err != nil {
		s.Fatal("Failed to install test apps: ", err)
	}
	defer wm.CloseApp(cleanupCtx)

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

func startTestPWAServer(ctx context.Context, wm *webapk.Manager, filesystem http.FileSystem) chan shareResult {
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

	wm.StartServer(ctx, mux, func(err error) {
		shareChan <- shareResult{err: errors.Wrap(err, "failed to start local server")}
	})

	return shareChan
}

func installTestApps(ctx context.Context, wm *webapk.Manager, a *arc.ARC) error {
	if err := wm.InstallPwa(ctx); err != nil {
		return errors.Wrap(err, "failed to install PWA")
	}
	if err := wm.InstallApk(ctx); err != nil {
		return errors.Wrap(err, "failed to install WebAPK")
	}

	const (
		testAPK = "ArcChromeWebApkTest.apk"
	)
	if err := a.Install(ctx, arc.APKPath(testAPK)); err != nil {
		return errors.Wrap(err, "failed to install test app")
	}

	return nil
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

	for i := 0; i < 2; i++ {
		select {
		case receivedShare = <-shareChan:
		case <-ctx.Done():
			return errors.New("timeout waiting to receive shared files")
		}

		// Occasionally, a second Intent is fired at ArcWebApkActivity after
		// clicking the "Share Text" button in the previous stage of the test.
		// This Intent causes a second text sharing request to be fired, before
		// progressing to sharing files.
		// It's not clear where this second Intent comes from, but it does not
		// seem to happen in non-test environments. Therefore, we allow one
		// text share to be received and ignored while waiting for a file
		// share. See crbug.com/1254586 for further details.
		if len(receivedShare.files) > 0 {
			break
		}
	}

	if receivedShare.err != nil {
		return errors.Wrap(receivedShare.err, "error received from test server")
	}

	if len(receivedShare.files) != 2 {
		return errors.Errorf("did not receive expected number of files: got %d, want: 2", len(receivedShare.files))
	}

	if receivedShare.files[0] != expectedFile0 {
		return errors.Errorf("failed to match shared file: got %q, want %q", receivedShare.files[0], expectedFile0)
	}
	if receivedShare.files[1] != expectedFile1 {
		return errors.Errorf("failed to match shared file: got %q, want %q", receivedShare.files[1], expectedFile1)
	}

	return nil
}
