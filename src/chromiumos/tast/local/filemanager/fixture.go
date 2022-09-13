// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filemanager exposes fixtures for Files app team tests.
package filemanager

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const (
	pwaInstallFixtureLocalServerPort = 8080
	pwaInstallFixtureInstallTimeout  = 15 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "install10Pwas",
		Desc:            "Installs 10 PWAs in ash chrome",
		Contacts:        []string{"chromeos-files-syd@chromium.org", "lucmult@chromium.org"},
		Parent:          "chromeLoggedIn",
		Impl:            &fixtureInstallPwa{numPwas: 10},
		Data:            []string{"pwa_manifest.json", "pwa_service.js", "pwa_index.html", "pwa_icon.png"},
		SetUpTimeout:    (pwaInstallFixtureInstallTimeout * 10),
		TearDownTimeout: 5 * time.Second,
	})
	testing.AddFixture(&testing.Fixture{
		Name:         "openFilesApp",
		Desc:         "Opens a Files app Window",
		Contacts:     []string{"chromeos-files-syd@chromium.org", "lucmult@chromium.org"},
		Parent:       "chromeLoggedIn",
		Impl:         &fixtureInstallPwa{numPwas: 0},
		SetUpTimeout: 10 * time.Second,
	})
}

type fixtureInstallPwa struct {
	cr          *chrome.Chrome
	server      *http.Server
	filesWindow *filesapp.FilesApp
	numPwas     int
}

// FixtureData is the struct exposed to tests.
type FixtureData struct {
	Chrome      *chrome.Chrome
	FilesWindow *filesapp.FilesApp
}

func (f *fixtureInstallPwa) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.cr = s.ParentValue().(*chrome.Chrome)
	cr := f.cr
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	startHTTPServer(f, s)
	const pwaWindowTitle = "PWA Open TXT Test App - Test PWA"

	// Install `numPwas` PWAs
	for appIdx := 1; appIdx <= f.numPwas; appIdx++ {
		if _, err := installPWAForURL(ctx, cr, fmt.Sprintf("http://127.0.0.%d:%v/pwa_index.html", appIdx, pwaInstallFixtureLocalServerPort), pwaInstallFixtureInstallTimeout); err != nil {
			s.Fatalf("Failed to install PWA %d, %s:", appIdx, err)
		}
		if _, err = ash.WaitForAnyWindowWithTitle(ctx, tconn, pwaWindowTitle); err != nil {
			s.Fatalf("Failed to wait for PWA window with title %s: %s", pwaWindowTitle, err)
		}
	}

	if f.numPwas > 0 {
		err = ash.CloseAllWindows(ctx, tconn)
		if err != nil {
			testing.ContextLog(ctx, "Error closing all PWA windows (continuing): ", err)
		}
	}

	// Launch the Files app.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	f.filesWindow = files
	return FixtureData{
		Chrome:      f.cr,
		FilesWindow: files,
	}
}

func (f *fixtureInstallPwa) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.server != nil {
		if err := f.server.Shutdown(ctx); err != nil {
			s.Error("Failed to stop http server: ", err)
		}
	}
}

func (f *fixtureInstallPwa) Reset(ctx context.Context) error                        { return nil }
func (f *fixtureInstallPwa) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *fixtureInstallPwa) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// installPWAForURL navigates to a PWA, attempts to install and returns the installed app ID.
func installPWAForURL(ctx context.Context, cr *chrome.Chrome, pwaURL string, timeout time.Duration) (string, error) {
	conn, err := cr.NewConn(ctx, pwaURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL %q", pwaURL)
	}
	defer conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to test API")
	}

	// The installability checks occur asynchronously for PWAs.
	// Wait for the Install button to appear in the Chrome omnibox before installing.
	ui := uiauto.New(tconn)
	install := nodewith.ClassName("PwaInstallView").Role(role.Button)
	if err := ui.WithTimeout(timeout).WaitUntilExists(install)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to wait for the install button in the omnibox")
	}

	// NOTE: This only installs the PWA in Ash.
	evalString := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.installPWAForCurrentURL)(%d)", timeout.Milliseconds())

	var appID string
	if err := tconn.Eval(ctx, evalString, &appID); err != nil {
		return "", errors.Wrap(err, "failed to run installPWAForCurrentURL")
	}

	return appID, nil
}

func startHTTPServer(f *fixtureInstallPwa, s *testing.FixtState) {
	mux := http.NewServeMux()
	fs := http.FileServer(s.DataFileSystem())
	mux.Handle("/", fs)

	server := &http.Server{Addr: fmt.Sprintf(":%v", pwaInstallFixtureLocalServerPort), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local server: ", err)
		}
	}()

	f.server = server
}
