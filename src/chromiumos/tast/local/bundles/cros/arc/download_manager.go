// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DownloadManager,
		Desc:         "Checks whether ARC can download files through DownloadManager",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-storage@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"capybara.jpg"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 7 * time.Minute,
	})
}

func DownloadManager(ctx context.Context, s *testing.State) {
	const (
		filename        = "capybara.jpg"
		localServerPort = 8080
	)

	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	sourcePath := s.DataPath(filename)

	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice
	cr := s.FixtValue().(*arc.PreData).Chrome

	// Create and start a local HTTP server that serves the test file data.
	mux := createServeMux(s, sourcePath)
	server := &http.Server{Addr: fmt.Sprintf(":%d", localServerPort), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to start a local server: ", err)
		}
	}()
	defer server.Shutdown(cleanupCtx)

	if err := waitForServerStart(ctx, localServerPort); err != nil {
		s.Fatal("Failed to wait for the server to start: ", err)
	}

	// Download the test file with an Android app from the local server.
	const targetPath = "/storage/emulated/0/Download/" + filename
	if err := downloadFileWithApp(ctx, cr, a, d, localServerPort, sourcePath, targetPath); err != nil {
		s.Fatal("Failed to download the test file with Android app: ", err)
	}
	defer func(ctx context.Context) {
		if err := a.RemoveAll(ctx, targetPath); err != nil {
			s.Fatalf("Failed to remove %s: %v", targetPath, err)
		}
	}(cleanupCtx)

	// Check whether the downloaded file is accessible from Chrome OS.
	original, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", sourcePath, err)
	}

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.User())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.User(), err)
	}
	targetPathInCros := filepath.Join(cryptohomeUserPath, "MyFiles", "Downloads", filename)

	downloaded, err := ioutil.ReadFile(targetPathInCros)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", targetPathInCros, err)
	}

	if !bytes.Equal(downloaded, original) {
		s.Fatalf("Content mismatch between the original file (%d bytes) and the downloaded file (%d bytes)", binary.Size(original), binary.Size(downloaded))
	}
}

// createServeMux returns an HTTP request multiplexer that serves the file in a
// given path. It also responds to a ping with the status code OK, which can be
// used to check whether a server is properly started.
func createServeMux(s *testing.State, path string) *http.ServeMux {
	mux := http.NewServeMux()

	// Register a handler that responds to a ping with OK.
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusBadRequest)
			s.Fatalf("Received a non-GET request: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Register a handler that serves the test file data.
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusBadRequest)
			s.Fatalf("Received a non-GET request: %s", r.Method)
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			s.Fatalf("Failed to read %s: %v", path, err)
		}
		w.Header().Set("Content-Length", strconv.Itoa(binary.Size(content)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			s.Fatalf("Failed to write the content of %s to ResponseWriter: %v", path, err)
		}
	})

	return mux
}

// waitForServerStart waits for a local HTTP server to start. It assumes that
// the server responds to a ping with the handler registered by createServeMux.
func waitForServerStart(ctx context.Context, localServerPort int) error {
	pingURL := fmt.Sprintf("http://localhost:%d/ping", localServerPort)
	return testing.Poll(ctx, func(ctx context.Context) error {
		r, err := http.Get(pingURL)
		if err != nil {
			return err
		}
		r.Body.Close()
		if r.StatusCode != http.StatusOK {
			return errors.Errorf("received a non-OK status code: %d", r.StatusCode)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// downloadFileWithApp downloads a file from sourcePath in Chrome OS to
// targetPath in Android with an Android app via a local HTTP server.
// It first sets up reverse port forwarding to connect the local server to an
// Android port, triggers the download from the connected Android port, and then
// returns when the download is completed.
func downloadFileWithApp(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, d *androidui.Device, localServerPort int, sourcePath, targetPath string) error {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Build the test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// Set up an Android port to which the host port is forwarded.
	androidPort, cleanupFunc, err := setUpAndroidPort(ctx, a, localServerPort)
	if err != nil {
		return errors.Wrap(err, "failed to set up Android port")
	}
	defer cleanupFunc(cleanupCtx)

	// Start downloading the test file.
	cleanupFunc, err = startDownloadFileWithApp(ctx, a, tconn, androidPort, sourcePath, targetPath)
	if err != nil {
		return errors.Wrap(err, "failed to start downloading the test file")
	}
	defer cleanupFunc(cleanupCtx)

	return waitForDownloadComplete(ctx, d)
}

// setUpAndroidPort sets up an Android port to which a specified host port is
// forwarded using reverse port forwarding.
func setUpAndroidPort(ctx context.Context, a *arc.ARC, localServerPort int) (int, func(context.Context), error) {
	androidPort, err := a.ReverseTCP(ctx, localServerPort)
	if err != nil {
		return -1, nil, errors.Wrap(err, "failed to start reverse port forwarding")
	}

	cleanupFunc := func(ctx context.Context) {
		if err := a.RemoveReverseTCP(ctx, androidPort); err != nil {
			testing.ContextLog(ctx, "Failed to stop reverse port forwarding: ", err)
		}
	}

	return androidPort, cleanupFunc, nil
}

// startDownloadFileWithApp starts to download a file from sourcePath in Chrome
// OS to targetPath in Android with an Android app via a specified Android port.
// It first installs the app, and then starts its MainActivity with an explicit
// intent to trigger the download.
func startDownloadFileWithApp(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, androidPort int, sourcePath, targetPath string) (func(context.Context), error) {
	const (
		apkName                        = "ArcDownloadManagerTest.apk"
		packageName                    = "org.chromium.arc.testapp.downloadmanager"
		writeExternalStoragePermission = "android.permission.WRITE_EXTERNAL_STORAGE"
		sourceURLKey                   = "source_url"
		targetPathKey                  = "target_path"
	)

	// Install the test app.
	if err := a.Install(ctx, arc.APKPath(apkName), adb.InstallOptionGrantPermissions); err != nil {
		return nil, errors.Wrapf(err, "failed to install %s", apkName)
	}

	// Create the MainActivity of the test app.
	act, err := arc.NewActivity(a, packageName, ".MainActivity")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create the main activity for %s", packageName)
	}

	// Start the MainActivity with an intent. When started successfully,
	// the test app automatically starts downloading the test file.
	sourceURL := fmt.Sprintf("http://localhost:%d%s", androidPort, sourcePath)
	startCommandPrefixes := []string{
		// Force stop and restart the target app if it's already started.
		// This allows us to start the activity always with a clean slate.
		"-S",
		// Wait for launch to complete.
		"-W",
		// Specify the component name to create an explicit intent.
		"-n",
	}
	startCommandSuffixes := []string{
		// Pass the source URL and target path as ExtraData of the intent.
		"--es", sourceURLKey, sourceURL,
		"--es", targetPathKey, targetPath,
	}
	if err := act.StartWithArgs(ctx, tconn, startCommandPrefixes, startCommandSuffixes); err != nil {
		act.Close()
		return nil, errors.Wrapf(err, "failed to start the main activity for %s", packageName)
	}

	cleanupFunc := func(ctx context.Context) {
		if err := act.Stop(ctx, tconn); err != nil {
			testing.ContextLogf(ctx, "Failed to stop the main activity for %s: %v", packageName, err)
		}
		act.Close()
	}

	return cleanupFunc, nil
}

// waitForDownloadComplete waits for a download session triggered by
// downloadFileWithApp to be completed. It checks the app's status field and
// returns when the "Finished" message appears on the field.
func waitForDownloadComplete(ctx context.Context, d *androidui.Device) error {
	const (
		appStatusFieldID  = "org.chromium.arc.testapp.downloadmanager:id/status"
		appStatusFinished = "Finished"
	)

	obj := d.Object(androidui.ID(appStatusFieldID))
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrapf(err, "failed to find the label id %s", appStatusFieldID)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		t, err := obj.GetText(ctx)
		if err != nil {
			return err
		}
		if t != appStatusFinished {
			return errors.Errorf("app status: %s", t)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for the app to finish downloading")
	}

	return nil
}
