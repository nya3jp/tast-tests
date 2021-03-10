// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUICoexistence,
		Desc:         "Verifies CCA can coexist with web page with camera open",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui_coexistence.html", "cca_ui_coexistence.js", "cca_ui.js"},
		Pre:          testutil.ChromeBypassCameraPermissions(),
	})
}

func CCAUICoexistence(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(cleanupCtx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	openCCAFunc := func() (*cca.App, error) {
		return cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	}

	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, func() (*cca.App, error), string) error
	}{
		{"testOpenCCAFirstAndCloseCCAFirst", testOpenCCAFirstAndCloseCCAFirst},
		{"testOpenCCAFirstAndCloseWebPageFirst", testOpenCCAFirstAndCloseWebPageFirst},
		{"testOpenWebPageFirstAndCloseCCAFirst", testOpenWebPageFirstAndCloseCCAFirst},
		{"testOpenWebPageFirstAndCloseWebPageFirst", testOpenWebPageFirstAndCloseWebPageFirst},
	} {
		s.Run(ctx, tst.name, func(ctx context.Context, s *testing.State) {
			if err := tst.testFunc(ctx, cr, openCCAFunc, server.URL+"/cca_ui_coexistence.html"); err != nil {
				s.Errorf("Subtest %v failed: %v", tst.name, err)
			}
		})
	}
}

func testOpenCCAFirstAndCloseCCAFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(ctx context.Context) {
		if app != nil {
			if err := app.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close CCA: ", err)
			}
		}
	}(cleanupCtx)

	pageConn, trackState, err := openWebPage(ctx, cr, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer func(ctx context.Context) {
		if pageConn != nil || trackState != nil {
			if err := closeWebPage(ctx, pageConn, trackState); err != nil {
				testing.ContextLog(ctx, "Failed to close web page: ", err)
			}
		}
	}(cleanupCtx)

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}
	app = nil

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}
	return nil
}

func testOpenCCAFirstAndCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(ctx context.Context) {
		if app != nil {
			if err := app.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close CCA: ", err)
			}
		}
	}(cleanupCtx)

	pageConn, trackState, err := openWebPage(ctx, cr, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer func(ctx context.Context) {
		if pageConn != nil || trackState != nil {
			if err := closeWebPage(ctx, pageConn, trackState); err != nil {
				testing.ContextLog(ctx, "Failed to close web page: ", err)
			}
		}
	}(cleanupCtx)

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}
	app = nil

	return nil
}

func testOpenWebPageFirstAndCloseCCAFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	pageConn, trackState, err := openWebPage(ctx, cr, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer func(ctx context.Context) {
		if pageConn != nil || trackState != nil {
			if err := closeWebPage(ctx, pageConn, trackState); err != nil {
				testing.ContextLog(ctx, "Failed to close web page: ", err)
			}
		}
	}(cleanupCtx)

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(ctx context.Context) {
		if app != nil {
			if err := app.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close CCA: ", err)
			}
		}
	}(cleanupCtx)

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}
	app = nil

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}
	return nil
}

func testOpenWebPageFirstAndCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	pageConn, trackState, err := openWebPage(ctx, cr, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer func(ctx context.Context) {
		if pageConn != nil || trackState != nil {
			if err := closeWebPage(ctx, pageConn, trackState); err != nil {
				testing.ContextLog(ctx, "Failed to close web page: ", err)
			}
		}
	}(cleanupCtx)

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(ctx context.Context) {
		if app != nil {
			if err := app.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close CCA: ", err)
			}
		}
	}(cleanupCtx)

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}
	app = nil

	return nil
}

func openWebPage(ctx context.Context, cr *chrome.Chrome, pageURL string) (_ *chrome.Conn, _ *chrome.JSObject, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	pageConn, err := cr.NewConn(ctx, pageURL)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create blank URL")
	}
	defer func(ctx context.Context) {
		if retErr != nil && pageConn != nil {
			if err := pageConn.CloseTarget(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close web page target: ", err)
			}
			if err := pageConn.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close web page connection: ", err)
			}
		}
	}(cleanupCtx)

	var trackState chrome.JSObject
	if err := pageConn.Call(ctx, &trackState, "Tast.startStream"); err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	return pageConn, &trackState, nil
}

func closeWebPage(ctx context.Context, pageConn *chrome.Conn, trackState *chrome.JSObject) error {
	var hasEnded bool
	var err error
	if err := trackState.Call(ctx, &hasEnded, "function() { return this.hasEnded; }"); err != nil {
		err = errors.Wrap(err, "failed to check track state")
	}
	if hasEnded {
		err = errors.Wrap(err, "failed as media track in web page unexpectedly ended")
	}

	if err := trackState.Release(ctx); err != nil {
		err = errors.Wrap(err, "failed to release track state")
	}
	trackState = nil

	if err := pageConn.CloseTarget(ctx); err != nil {
		err = errors.Wrap(err, "failed to close web page target")
	}

	if err := pageConn.Close(); err != nil {
		err = errors.Wrap(err, "failed to close web page connection")
	}
	pageConn = nil

	return err
}
