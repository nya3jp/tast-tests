// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type params struct {
	expectedID, resultID string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: TransferFromImageBitmapOrientation,
		Desc: "Verifies that transferFromImageBitmap is oriented correctly",
		Contacts: []string{
			"aswolfers@chromium.org",
			"chromeos-gfx-compositor@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"transfer-from-image-bitmap.html"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "2d",
			Val: params{
				expectedID: "expected2d",
				resultID:   "result2d",
			},
		}, {
			Name: "webgl",
			Val: params{
				expectedID: "expectedWebGL",
				resultID:   "resultWebGL",
			},
		}},
	})
}

// TransferFromImageBitmapOrientation renders to offscreen canvases, transfers their contents to
// bitmap renderers, and screenshots the results. The screenshots are compared against screenshots
// of reference canvases to validate that the results are oriented correctly.
func TransferFromImageBitmapOrientation(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	if err := graphics.RotateDisplayToLandscapePrimary(ctx, tconn); err != nil {
		s.Fatal("Failed to set display to landscape-primary orientation: ", err)
	}

	url := path.Join(server.URL, "transfer-from-image-bitmap.html")
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", url, err)
	}
	defer conn.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize the keyboard writer: ", err)
	}
	defer kb.Close()

	// screenshotCanvas requests a fullscreen view of the specified element, takes a screenshot,
	// and returns the resulting image.
	screenshotCanvas := func(id string) (image.Image, error) {
		if err := conn.Call(ctx, nil, "setFullscreenEventListener", id); err != nil {
			return nil, errors.Wrapf(err,
				"failed to set fullscreen event listener on element %v", id)
		}
		if err := kb.Type(ctx, "f"); err != nil {
			return nil, errors.Wrap(err, "failed to inject the 'f' key")
		}
		sshotPath := filepath.Join(s.OutDir(), fmt.Sprintf("%v.png", id))
		if err := screenshot.Capture(ctx, sshotPath); err != nil {
			return nil, errors.Wrapf(err, "failed to capture screenshot of element %v",
				id)
		}
		f, err := os.Open(sshotPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open %v", sshotPath)
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode %v", sshotPath)
		}
		return img, nil
	}

	params := s.Param().(params)
	expectedImg, err := screenshotCanvas(params.expectedID)
	if err != nil {
		s.Fatalf("Failed to screenshot canvas %v: %v", params.expectedID, err)
	}
	resultImg, err := screenshotCanvas(params.resultID)
	if err != nil {
		s.Fatalf("Failed to screenshot canvas %v: %v", params.resultID, err)
	}

	if expectedImg.Bounds() != resultImg.Bounds() {
		s.Fatalf("Mismatched image bounds: Expected %v, Actual %v", expectedImg.Bounds(),
			resultImg.Bounds())
	}

	// Compare colors of pixels at the center of each image quadrant to verify orientation.
	// Note that on non-square displays, the screenshots will have transparent margins. Correct
	// for this by positioning each sample relative to the image center, offset by 1/4 of the
	// smaller image dimension (which should generally be height in landscape mode).
	width := expectedImg.Bounds().Dx()
	height := expectedImg.Bounds().Dy()
	cX := width / 2
	cY := height / 2
	offset := height / 4
	if width < height {
		offset = width / 4
	}
	samples := []image.Point{
		{cX - offset, cY - offset},
		{cX - offset, cY + offset},
		{cX + offset, cY - offset},
		{cX + offset, cY + offset},
	}

	for _, sample := range samples {
		expectedColor := expectedImg.At(sample.X, sample.Y)
		resultColor := resultImg.At(sample.X, sample.Y)
		if expectedColor != resultColor {
			s.Errorf("Mismatched colors at (%d, %d): Expected %v, Actual %v", sample.X,
				sample.Y, expectedColor, resultColor)
		}
	}
}
