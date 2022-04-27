// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testpage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUICoexistence,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies CCA can coexist with web page with camera open",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"camera_page.html", "camera_page.js", "cca_ui.js"},
		Fixture:      "ccaTestBridgeReadyBypassPermissionClamshell",
	})
}

func CCAUICoexistence(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr := s.FixtValue().(cca.FixtureData).Chrome
	openCCA := s.FixtValue().(cca.FixtureData).StartApp
	closeCCA := s.FixtValue().(cca.FixtureData).StopApp

	pageGen := func() *testpage.CameraWebPage {
		return testpage.New(server.URL)
	}

	subTestTimeout := 30 * time.Second
	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, context.Context, *chrome.Chrome, cca.StartAppFunc, cca.StopAppFunc, func() *testpage.CameraWebPage) error
	}{
		{"testOpenCCAFirstAndCloseCCAFirst", testOpenCCAFirstAndCloseCCAFirst},
		{"testOpenCCAFirstAndCloseWebPageFirst", testOpenCCAFirstAndCloseWebPageFirst},
		{"testOpenWebPageFirstAndCloseCCAFirst", testOpenWebPageFirstAndCloseCCAFirst},
		{"testOpenWebPageFirstAndCloseWebPageFirst", testOpenWebPageFirstAndCloseWebPageFirst},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			if err := cca.ClearSavedDir(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			if err := tst.testFunc(cleanupCtx, ctx, cr, openCCA, closeCCA, pageGen); err != nil {
				s.Errorf("Failed to run subtest %v: %v", tst.name, err)
			}
		})
		cancel()
	}
}

func testOpenCCAFirstAndCloseCCAFirst(cleanupCtx, ctx context.Context, cr *chrome.Chrome, openCCA cca.StartAppFunc, closeCCA cca.StopAppFunc, newCameraWebPage func() *testpage.CameraWebPage) (retErr error) {
	_, err := openCCA(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(cleanupCtx context.Context) {
		if err := closeCCA(cleanupCtx, retErr != nil); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
	}(cleanupCtx)

	webPage := newCameraWebPage()
	if err := webPage.Open(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(cleanupCtx)

	if err := closeCCA(ctx, false); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	return nil
}

func testOpenCCAFirstAndCloseWebPageFirst(cleanupCtx, ctx context.Context, cr *chrome.Chrome, openCCA cca.StartAppFunc, closeCCA cca.StopAppFunc, newCameraWebPage func() *testpage.CameraWebPage) (retErr error) {
	_, err := openCCA(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(cleanupCtx context.Context) {
		if err := closeCCA(cleanupCtx, retErr != nil); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
	}(cleanupCtx)

	webPage := newCameraWebPage()
	if err := webPage.Open(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(cleanupCtx)

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	if err := closeCCA(ctx, false); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	return nil
}

func testOpenWebPageFirstAndCloseCCAFirst(cleanupCtx, ctx context.Context, cr *chrome.Chrome, openCCA cca.StartAppFunc, closeCCA cca.StopAppFunc, newCameraWebPage func() *testpage.CameraWebPage) (retErr error) {
	webPage := newCameraWebPage()
	if err := webPage.Open(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(cleanupCtx)

	_, err := openCCA(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(cleanupCtx context.Context) {
		if err := closeCCA(cleanupCtx, retErr != nil); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
	}(cleanupCtx)

	if err := closeCCA(ctx, false); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	return nil
}

func testOpenWebPageFirstAndCloseWebPageFirst(cleanupCtx, ctx context.Context, cr *chrome.Chrome, openCCA cca.StartAppFunc, closeCCA cca.StopAppFunc, newCameraWebPage func() *testpage.CameraWebPage) (retErr error) {
	webPage := newCameraWebPage()
	if err := webPage.Open(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to open web page")
	}
	defer webPage.Close(cleanupCtx)

	_, err := openCCA(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(cleanupCtx context.Context) {
		if err := closeCCA(cleanupCtx, retErr != nil); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
	}(cleanupCtx)

	if err := webPage.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close web page")
	}

	if err := closeCCA(ctx, false); err != nil {
		return errors.Wrap(err, "failed to close CCA")
	}

	return nil
}
