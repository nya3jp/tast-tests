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
			if err := tst.testFunc(cleanupCtx, cr, openCCAFunc, server.URL+"/cca_ui_coexistence.html"); err != nil {
				s.Errorf("Failed to run subtest %v: %v", tst.name, err)
			}
		})
	}
}

func testOpenCCAFirstAndCloseCCAFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	pageConn, err := cr.NewConn(ctx, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open page")
	}
	defer closeWebPage(ctx, pageConn, true)

	trackState, err := startStream(ctx, pageConn)
	if err != nil {
		return errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	defer checkAndCloseStream(ctx, trackState, true)

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	if err := checkAndCloseStream(ctx, trackState, false); err != nil {
		return errors.Wrap(err, "failed to close stream")
	}
	trackState = nil

	if err := closeWebPage(ctx, pageConn, false); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}
	pageConn = nil

	return nil
}

func testOpenCCAFirstAndCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	pageConn, err := cr.NewConn(ctx, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open page")
	}
	defer closeWebPage(ctx, pageConn, true)

	trackState, err := startStream(ctx, pageConn)
	if err != nil {
		return errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	defer checkAndCloseStream(ctx, trackState, true)

	if err := checkAndCloseStream(ctx, trackState, false); err != nil {
		return errors.Wrap(err, "failed to close stream")
	}
	trackState = nil

	if err := closeWebPage(ctx, pageConn, false); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}
	pageConn = nil

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	return nil
}

func testOpenWebPageFirstAndCloseCCAFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	pageConn, err := cr.NewConn(ctx, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open page")
	}
	defer closeWebPage(ctx, pageConn, true)

	trackState, err := startStream(ctx, pageConn)
	if err != nil {
		return errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	defer checkAndCloseStream(ctx, trackState, true)

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	if err := checkAndCloseStream(ctx, trackState, false); err != nil {
		return errors.Wrap(err, "failed to close stream")
	}
	trackState = nil

	if err := closeWebPage(ctx, pageConn, false); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}
	pageConn = nil

	return nil
}

func testOpenWebPageFirstAndCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	pageConn, err := cr.NewConn(ctx, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open page")
	}
	defer closeWebPage(ctx, pageConn, true)

	trackState, err := startStream(ctx, pageConn)
	if err != nil {
		return errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	defer checkAndCloseStream(ctx, trackState, true)

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	if err := checkAndCloseStream(ctx, trackState, false); err != nil {
		return errors.Wrap(err, "failed to close stream")
	}
	trackState = nil

	if err := closeWebPage(ctx, pageConn, false); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}
	pageConn = nil

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	return nil
}

func startStream(ctx context.Context, pageConn *chrome.Conn) (*chrome.JSObject, error) {
	var trackState chrome.JSObject
	if err := pageConn.Call(ctx, &trackState, "Tast.startStream"); err != nil {
		return nil, errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	return &trackState, nil
}

func closeWebPage(ctx context.Context, pageConn *chrome.Conn, logError bool) error {
	if pageConn == nil {
		return nil
	}

	var retErr error
	if err := pageConn.CloseTarget(ctx); err != nil {
		retErr = errors.Wrap(err, "failed to close web page target")
	}
	if err := pageConn.Close(); err != nil {
		if retErr != nil {
			testing.ContextLog(ctx, "Failed to close web page connection: ", err)
		} else {
			retErr = errors.Wrap(err, "failed to close web page connection")
		}
	}

	if retErr != nil && logError {
		testing.ContextLog(ctx, "Failed to close web page: ", retErr)
	}
	return retErr
}

func checkAndCloseStream(ctx context.Context, trackState *chrome.JSObject, logError bool) error {
	if trackState == nil {
		return nil
	}

	var hasEnded bool
	var retErr error
	err := trackState.Call(ctx, &hasEnded, "function() { return this.hasEnded; }")
	if err != nil {
		retErr = errors.Wrap(err, "failed to check track state")
	} else if hasEnded {
		retErr = errors.Wrap(err, "failed as media track in web page unexpectedly ended")
	}
	if err := trackState.Release(ctx); err != nil {
		if retErr != nil {
			testing.ContextLog(ctx, "Failed to release track state: ", err)
		} else {
			retErr = errors.Wrap(err, "failed to release track state")
		}
	}

	if retErr != nil && logError {
		testing.ContextLog(ctx, "Failed to check and close stream: ", err)
	}
	return retErr
}
