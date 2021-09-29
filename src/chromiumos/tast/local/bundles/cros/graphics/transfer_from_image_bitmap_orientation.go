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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

type params struct {
	expectedID, resultID string
}

type vector struct {
	x, y float32
}

func init() {
	testing.AddTest(&testing.Test{
		Func: TransferFromImageBitmapOrientation,
		Desc: "Verifies that ImageBitmapRenderer.trasferFromImageBitmap results in correctly oriented output when transferred from various rendering contexts",
		Contacts: []string{
			"aswolfers@chromium.org",
			"chromeos-gfx-compositor@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"transfer-from-image-bitmap.html"},
		SoftwareDeps: []string{"chrome"},
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
				expectedID: "expectedWebgl",
				resultID:   "resultWebgl",
			},
		}},
	})
}

// TransferFromImageBitmapOrientation draws to offscreen canvases and transfers their contents to bitmap renderers, and compares the results against reference canvases to validate that the results are oriented correctly.
func TransferFromImageBitmapOrientation(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
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

	// screenshotCanvas requests a fullscreen view of the specified element, takes a screenshot, and returns the resulting image.
	screenshotCanvas := func(id string) (image.Image, error) {
		if err := conn.Call(ctx, nil, "setFullscreenEventListener", id); err != nil {
			return nil, errors.Wrapf(err, "failed to set fullscreen event listener on element %v", id)
		}
		if err := kb.Type(ctx, "f"); err != nil {
			return nil, errors.Wrap(err, "failed to inject the 'f' key")
		}
		sshotPath := filepath.Join(s.OutDir(), fmt.Sprintf("%v.png", id))
		if err := screenshot.Capture(ctx, sshotPath); err != nil {
			return nil, errors.Wrapf(err, "failed to capture screenshot of element %v", id)
		}
		f, err := os.Open(sshotPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open %v", sshotPath)
		}
		img, _, err := image.Decode(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode %v", sshotPath)
		}
		if err := f.Close(); err != nil {
			return nil, errors.Wrapf(err, "failed to close %v", sshotPath)
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
		s.Fatalf("Mismatched image bounds: Expected %v, Actual %v", expectedImg.Bounds(), resultImg.Bounds())
	}

	// Compare colors of pixels at the center of each image quadrant to verify orientation.
	samples := []vector{{0.25, 0.25}, {0.25, 0.75}, {0.75, 0.25}, {0.75, 0.75}}
	for _, scale := range samples {
		x := int(scale.x * float32(expectedImg.Bounds().Dx()))
		y := int(scale.y * float32(expectedImg.Bounds().Dy()))
		expectedColor := expectedImg.At(x, y)
		resultColor := resultImg.At(x, y)
		if expectedColor != resultColor {
			s.Errorf("Mismatched colors at (%d, %d): Expected %v, Actual %v", x, y, expectedColor, resultColor)
		}
	}

	if s.HasError() {
		p := perf.NewValues()
		p.Save(s.OutDir())
	}
}
