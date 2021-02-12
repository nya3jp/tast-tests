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
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DownloadManager,
		Desc:         "Checks whether ARC can download files through DownloadManager",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-storage@google.com"},
		Attr:         []string{"group:mainline", "informational"},
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

	const targetPath = "/storage/emulated/0/Download/" + filename
	if err := downloadFileWithApp(ctx, cr, a, localServerPort, sourcePath, targetPath); err != nil {
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
	targetPathInCros := cryptohomeUserPath + "/MyFiles/Downloads/" + filename

	downloaded, err := ioutil.ReadFile(targetPathInCros)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", targetPathInCros, err)
	}

	if !bytes.Equal(downloaded, original) {
		s.Fatal("Content mismatch between the original file and the downloaded file")
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
func downloadFileWithApp(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, localServerPort int, sourcePath, targetPath string) (retErr error) {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Build the test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// Set up reverse port forwarding.
	// This is needed for the test app to access the host's local server.
	androidPort, err := a.ReverseTCP(ctx, localServerPort)
	if err != nil {
		return errors.Wrap(err, "failed to start reverse port forwarding")
	}
	defer func(ctx context.Context, androidPort int) {
		if err := a.RemoveReverseTCP(ctx, androidPort); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop reverse port forwarding")
			} else {
				testing.ContextLog(ctx, "Failed to stop reverse port forwarding: ", err)
			}
		}
	}(cleanupCtx, androidPort)

	return downloadFileWithAppFromLocalAndroidPort(ctx, a, tconn, androidPort, sourcePath, targetPath)
}

func downloadFileWithAppFromLocalAndroidPort(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, androidPort int, sourcePath, targetPath string) (retErr error) {
	const (
		apkName                        = "ArcDownloadManagerTest.apk"
		packageName                    = "org.chromium.arc.testapp.downloadmanager"
		writeExternalStoragePermission = "android.permission.WRITE_EXTERNAL_STORAGE"
		sourceURLKey                   = "source_url"
		targetPathKey                  = "target_path"
	)

	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Install the test app.
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		return errors.Wrapf(err, "failed to install %s", apkName)
	}
	defer func(ctx context.Context) {
		if err := a.Uninstall(ctx, packageName); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed to uninstall %s", packageName)
			} else {
				testing.ContextLogf(ctx, "Failed to uninstall %s: %v", packageName, err)
			}
		}
	}(cleanupCtx)

	// Grant the WRITE_EXTERNAL_STORAGE permission to the test app.
	// This is needed when running the test for ARC++ P.
	if err := a.Command(ctx, "pm", "grant", packageName, writeExternalStoragePermission).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to grant %s to %s", writeExternalStoragePermission, packageName)
	}

	// Create the MainActivity of the test app.
	act, err := arc.NewActivity(a, packageName, ".MainActivity")
	if err != nil {
		return errors.Wrapf(err, "failed to create the main activity for %s", packageName)
	}
	defer act.Close()

	// Start the MainActivity with an intent. When started successfully,
	// the test app automatically starts downloading the test file.
	sourceURL := fmt.Sprintf("http://localhost:%d%s", androidPort, sourcePath)
	startCommandPrefixes := []string{
		"-S", // Force stop and restart the target app if it's already started.
		"-W", // Wait for launch to complete.
		"-n", // Specify the component name to create an explicit intent.
	}
	startCommandSuffixes := []string{
		// Pass the source URL and target path as ExtraData of the intent.
		"--es", sourceURLKey, sourceURL,
		"--es", targetPathKey, targetPath,
	}
	if err := act.StartWithArgs(ctx, tconn, startCommandPrefixes, startCommandSuffixes); err != nil {
		return errors.Wrapf(err, "failed to start the main activity for %s", packageName)
	}
	defer func(ctx context.Context, tconn *chrome.TestConn) {
		if err := act.Stop(ctx, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed to stop the main activity for %s", packageName)
			} else {
				testing.ContextLogf(ctx, "Failed to stop the main activity for %s: %v", packageName, err)
			}
		}
	}(cleanupCtx, tconn)

	return waitForDownloadComplete(ctx, a)
}

// waitForDownloadComplete waits for a download session triggered by
// downloadFileWithApp to be completed. It checks the app's status field and
// returns when the "Finished" message appears on the field.
func waitForDownloadComplete(ctx context.Context, a *arc.ARC) error {
	const (
		appStatusFieldID  = "org.chromium.arc.testapp.downloadmanager:id/status"
		appStatusFinished = "Finished"
	)

	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(cleanupCtx)

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
