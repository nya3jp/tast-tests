// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"encoding/binary"
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
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"capybara.jpg"},
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 7 * time.Minute,
	})
}

func DownloadManager(ctx context.Context, s *testing.State) {
	const (
		filename        = "capybara.jpg"
		localServerPort = "8080"
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
	server := &http.Server{Addr: ":" + localServerPort, Handler: mux}
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
	defer func() {
		if err := a.Command(cleanupCtx, "rm", targetPath).Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Failed to remove %s via adb: %v", targetPath, err)
		}
	}()

	// Check whether the downloaded file is accessible from Chrome OS.
	original, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", sourcePath, err)
	}

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.User())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.User(), err)
	}

	targetPathInCrOS := cryptohomeUserPath + "/MyFiles/Downloads/" + filename

	downloaded, err := ioutil.ReadFile(targetPathInCrOS)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", targetPathInCrOS, err)
	}

	if !bytes.Equal(downloaded, original) {
		s.Fatal("Content mismatch between the original file and the downloaded file")
	}
}

// createServeMux returns an HTTP request multiplexer that serves the file in a
// given |path|. It also responds to a ping with the status code OK, which can
// be used to check whether a server is properly started.
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
func waitForServerStart(ctx context.Context, localServerPort string) error {
	pingURL := "http://localhost:" + localServerPort + "/ping"
	return testing.Poll(ctx, func(ctx context.Context) error {
		r, err := http.Get(pingURL)
		if err != nil {
			return err
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			return errors.Errorf("received a non-OK status code: %d", r.StatusCode)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// downloadFileWithApp downloads a file in |sourcePath| in Chrome OS to
// |targetPath| in Android with an Android app via a local HTTP server.
// The app downloads the file through DownloadManager.
func downloadFileWithApp(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, localServerPort, sourcePath, targetPath string) (retErr error) {
	const (
		APKName       = "ArcDownloadManagerTest.apk"
		packageName   = "org.chromium.arc.testapp.downloadmanager"
		sourceURLKey  = "source_url"
		targetPathKey = "target_path"
	)

	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up reverse port forwarding.
	tcpPort := "tcp:" + localServerPort
	if err := a.ADBCommand(ctx, "reverse", tcpPort, tcpPort).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to start reverse port forwarding")
	}
	defer func() {
		if err := a.ADBCommand(cleanupCtx, "reverse", "--remove", tcpPort).Run(testexec.DumpLogOnError); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop reverse port forwarding")
			} else {
				testing.ContextLog(cleanupCtx, "Failed to stop reverse port forwarding: ", err)
			}
		}
	}()

	// Install and launch the test app.
	if err := a.Install(ctx, arc.APKPath(APKName)); err != nil {
		return errors.Wrapf(err, "failed to install %s", APKName)
	}

	act, err := arc.NewActivity(a, packageName, ".MainActivity")
	if err != nil {
		return errors.Wrapf(err, "failed to create the main activity for %s", packageName)
	}
	defer act.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	sourceURL := "http://localhost:" + localServerPort + sourcePath
	startCommandPrefixes := []string{
		"-S", // Force stop the target app before starting the activity.
		"-W", // Wait for launch to complete.
		"-n", // Specify the component name to create an explicit intent.
	}
	startCommandSuffixes := []string{
		// Pass the source URL and target path as ExtraData for the intent.
		"--es", sourceURLKey, sourceURL,
		"--es", targetPathKey, targetPath,
	}
	if err := act.StartWithArgs(ctx, tconn, startCommandPrefixes, startCommandSuffixes); err != nil {
		return errors.Wrapf(err, "failed to start the main activity for %s", packageName)
	}
	defer func() {
		if err := act.Stop(cleanupCtx, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed to stop the main activity for %s", packageName)
			} else {
				testing.ContextLogf(cleanupCtx, "Failed to stop the main activity for %s: %v", packageName, err)
			}
		}
	}()

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
