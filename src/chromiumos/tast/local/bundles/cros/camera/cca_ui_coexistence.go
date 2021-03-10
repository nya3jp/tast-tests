// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

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
		Data:         []string{"cca_ui.js"},
		Pre:          testutil.ChromeBypassCameraPermissions(),
	})
}

func CCAUICoexistence(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, func() (*cca.App, error)) error
	}{
		{"testOpenCCAFirstCloseCCAFirst", testOpenCCAFirstCloseCCAFirst},
		{"testOpenCCAFirstCloseWebPageFirst", testOpenCCAFirstCloseWebPageFirst},
		{"testOpenWebPageFirstCloseCCAFirst", testOpenWebPageFirstCloseCCAFirst},
		{"testOpenWebPageFirstCloseWebPageFirst", testOpenWebPageFirstCloseWebPageFirst},
	} {
		s.Run(ctx, tst.name, func(ctx context.Context, s *testing.State) {
			openCCAFunc := func() (*cca.App, error) {
				return cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
			}
			if err := tst.testFunc(ctx, cr, openCCAFunc); err != nil {
				s.Error("Subtest failed: ", err)
			}
		})
	}
}

func testOpenCCAFirstCloseCCAFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error)) error {
	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}

	pageConn, trackState, err := openWebPage(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}

	if err := closeCCA(ctx, app); err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	return nil
}

func testOpenCCAFirstCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error)) error {
	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}

	pageConn, trackState, err := openWebPage(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}

	if err := closeCCA(ctx, app); err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	return nil
}

func testOpenWebPageFirstCloseCCAFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error)) error {
	pageConn, trackState, err := openWebPage(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}

	if err := closeCCA(ctx, app); err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	return nil
}

func testOpenWebPageFirstCloseWebPageFirst(ctx context.Context, cr *chrome.Chrome, openCCA func() (*cca.App, error)) error {
	pageConn, trackState, err := openWebPage(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open web page")
	}

	app, err := openCCA()
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}

	if err := closeWebPage(ctx, pageConn, trackState); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}

	if err := closeCCA(ctx, app); err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	return nil
}

func openWebPage(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, *chrome.JSObject, error) {
	pageConn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create blank URL")
	}

	var trackState chrome.JSObject
	const code = `
	  async function() {
			const stream = await navigator.mediaDevices.getUserMedia({audio: false, video: true});
			const track = stream.getVideoTracks()[0];
			const trackState = { hasEnded: false };
			track.addEventListener('ended', () => {
				trackState.hasEnded = true;
			});
			return trackState;
	  }`
	if err := pageConn.Call(ctx, &trackState, code); err != nil {
		return nil, nil, errors.Wrap(err, "failed to run getUserMedia()")
	}
	return pageConn, &trackState, nil
}

func closeCCA(ctx context.Context, app *cca.App) error {
	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}
	return nil
}

func closeWebPage(ctx context.Context, pageConn *chrome.Conn, trackState *chrome.JSObject) error {
	var hasEnded bool
	if err := trackState.Call(ctx, &hasEnded, "function() { return this.hasEnded; }"); err != nil {
		return errors.Wrap(err, "failed to check track state")
	}
	if hasEnded {
		return errors.New("Media track in web page is unexpectedly ended")
	}

	if err := trackState.Release(ctx); err != nil {
		return errors.Wrap(err, "failed to release track state")
	}

	if err := pageConn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page target")
	}

	if err := pageConn.Close(); err != nil {
		return errors.Wrap(err, "failed to close web page connection")
	}
	return nil
}
