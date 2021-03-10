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

// cameraWebPage holds all connections to the web page which opens a camera stream.
type cameraWebPage struct {
	pageConn   *chrome.Conn
	trackState *chrome.JSObject
}

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

	var webPage cameraWebPage
	if err := webPage.Open(ctx, cr, pageURL); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(ctx)

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	return nil
}

func testOpenCCAFirstAndCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	var webPage cameraWebPage
	if err := webPage.Open(ctx, cr, pageURL); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(ctx)

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	return nil
}

func testOpenWebPageFirstAndCloseCCAFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	var webPage cameraWebPage
	if err := webPage.Open(ctx, cr, pageURL); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(ctx)

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	return nil
}

func testOpenWebPageFirstAndCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error), pageURL string) error {
	var webPage cameraWebPage
	if err := webPage.Open(ctx, cr, pageURL); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(ctx)

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	return nil
}

func (w *cameraWebPage) Open(ctx context.Context, cr *chrome.Chrome, pageURL string) (retErr error) {
	var err error
	w.pageConn, err = cr.NewConn(ctx, pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open page")
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			if err := w.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close web page: ", err)
			}
		}
	}(ctx)

	var trackState chrome.JSObject
	if err := w.pageConn.Call(ctx, &trackState, "Tast.startStream"); err != nil {
		return errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	w.trackState = &trackState
	return nil
}

func (w *cameraWebPage) Close(ctx context.Context) (retErr error) {
	if w.trackState != nil {
		var hasEnded bool
		err := w.trackState.Call(ctx, &hasEnded, "function() { return this.hasEnded; }")
		if err != nil {
			retErr = appendError(retErr, err, "failed to check track state")
		} else if hasEnded {
			retErr = appendError(retErr, nil, "failed as media track in web page unexpectedly ended")
		}
		if err := w.trackState.Release(ctx); err != nil {
			retErr = appendError(retErr, err, "failed to release track state")
		}
		w.trackState = nil
	}
	if w.pageConn != nil {
		if err := w.pageConn.CloseTarget(ctx); err != nil {
			retErr = appendError(retErr, err, "failed to close web page target")
		}
		if err := w.pageConn.Close(); err != nil {
			retErr = appendError(retErr, err, "failed to close web page connection")
		}
		w.pageConn = nil
	}
	return retErr
}

func appendError(err, newErr error, msg string) error {
	if err == nil && newErr == nil {
		return errors.New(msg)
	}
	if err == nil {
		return errors.Wrap(newErr, msg)
	} else if newErr == nil {
		return errors.Wrap(err, msg)
	}
	return errors.Wrapf(err, "%v, %v", msg, newErr.Error())
}
