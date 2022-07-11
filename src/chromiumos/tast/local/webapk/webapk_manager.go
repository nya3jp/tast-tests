// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webapk

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
)

// WebAPK is used to represent a ChromeOS WebApk app.
type WebAPK struct {
	// Name is the corresponding app name.
	Name string
	// Name is the corresponding app name.
	ID string
	// Port is the port the web server will expose.
	// It is hardcoded in the corresponding WebAPK.
	Port int
	// ApkDataPath is the pre-generated WebAPK data path which points to the app.
	ApkDataPath string
	// IndexPageDataPath is the index page of the app.
	IndexPageDataPath string
}

// Manager helps manage a WebAPK.
type Manager struct {
	arc    *arc.ARC
	cr     *chrome.Chrome
	br     *browser.Browser
	dpr    DataPathResolver
	server *http.Server
	tconn  *chrome.TestConn
	webapk WebAPK
}

// DataPathResolver helps resolve DataPath.
// Both testing.State and testing.FixtState match it.
type DataPathResolver interface {
	DataPath(p string) string
}

// NewManager returns a reference to a new Manager.
func NewManager(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, arc *arc.ARC, dpr DataPathResolver, webapk WebAPK) (*Manager, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a Test API connection")
	}

	return &Manager{
		arc:    arc,
		cr:     cr,
		br:     br,
		dpr:    dpr,
		server: nil,
		tconn:  tconn,
		webapk: webapk,
	}, nil
}

// StartServerFromDir starts an http server and serves files from the provided directory.
func (wm *Manager) StartServerFromDir(ctx context.Context, dir string, onError func(error)) {
	fsHandler := http.FileServer(http.Dir(dir))
	wm.StartServer(ctx, fsHandler, onError)
}

// StartServer starts an http server with a custom request handler (e.g. http.NewServeMux).
// Accepts a custom error handler.
func (wm *Manager) StartServer(ctx context.Context, handler http.Handler, onError func(error)) {
	server := &http.Server{Addr: fmt.Sprintf(":%d", wm.webapk.Port), Handler: handler}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			onError(err)
		} else {
			wm.server = nil
		}
	}()

	wm.server = server
}

// ShutdownServer shuts down an active http server.
func (wm *Manager) ShutdownServer(ctx context.Context) error {
	if wm.server == nil {
		return errors.Errorf("failed to shut down the http server for %q because it hasn't been started or has already been shut down", wm.webapk.Name)
	}

	if err := wm.server.Shutdown(ctx); err != nil {
		return errors.Wrapf(err, "failed to shut down http server for %q", wm.webapk.Name)
	}

	return nil
}

// InstallPwa installs the corresponding PWA. If fails, attempts to uninstall a pre-exist PWA
// and install it again.
func (wm *Manager) InstallPwa(ctx context.Context) error {
	installTimeout := 15 * time.Second
	localServerIndex := fmt.Sprintf(`http://localhost:%d/%s`, wm.webapk.Port, wm.webapk.IndexPageDataPath)

	if err := apps.InstallPWAForURL(ctx, wm.tconn, wm.br, localServerIndex, installTimeout); err != nil {
		if errUninstall := wm.UninstallPwa(ctx); errUninstall != nil {
			return errors.Wrapf(errUninstall, "failed to uninstall the pre-exist PWA %q", wm.webapk.Name)
		}
		if err := apps.InstallPWAForURL(ctx, wm.tconn, wm.br, localServerIndex, installTimeout); err != nil {
			return errors.Wrapf(err, "failed to install PWA %q", wm.webapk.Name)
		}
	}

	if err := ash.WaitForChromeAppInstalled(ctx, wm.tconn, wm.webapk.ID, installTimeout); err != nil {
		return errors.Wrapf(err, "failed to wait for PWA %q to be installed", wm.webapk.Name)
	}

	return nil
}

// UninstallPwa uninstalls the corresponding PWA.
func (wm *Manager) UninstallPwa(ctx context.Context) error {
	if err := ossettings.UninstallApp(ctx, wm.tconn, wm.cr, wm.webapk.Name, wm.webapk.ID); err != nil {
		return errors.Wrapf(err, "failed to uninstall the PWA %q", wm.webapk.Name)
	}

	return nil
}

// InstallApk installs the corresponding WebAPK.
func (wm *Manager) InstallApk(ctx context.Context) error {
	webAPKPath := wm.dpr.DataPath(wm.webapk.ApkDataPath)
	if err := wm.arc.Install(ctx, webAPKPath); err != nil {
		return errors.Wrapf(err, "failed to install the app from %q", wm.webapk.ApkDataPath)
	}

	return nil
}

// LaunchApp launches the corresponding WebAPK.
func (wm *Manager) LaunchApp(ctx context.Context) error {
	if err := apps.Launch(ctx, wm.tconn, wm.webapk.ID); err != nil {
		return errors.Wrapf(err, "failed launching the app with ID %q", wm.webapk.ID)
	}

	return nil
}

// CloseApp closes the corresponding WebAPK.
func (wm *Manager) CloseApp(ctx context.Context) error {
	if err := apps.Close(ctx, wm.tconn, wm.webapk.ID); err != nil {
		return errors.Wrapf(err, "failed to close the app with ID %q", wm.webapk.ID)
	}

	return nil
}

// GetChromeConnection creates a new chrome connection for the PWA and returns it.
func (wm *Manager) GetChromeConnection(ctx context.Context) (*chrome.Conn, error) {
	localServerAddress := fmt.Sprintf("http://127.0.0.1:%d/", wm.webapk.Port)

	newConn, err := wm.br.NewConnForTarget(ctx, chrome.MatchTargetURL(localServerAddress))
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting connection for target: %q", localServerAddress)
	}

	return newConn, nil
}
