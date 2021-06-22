// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type drawOnCanvasParams struct {
	fileName    string
	refFileName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DrawOnCanvas,
		Desc: "Verifies that a video can be drawn once onto a 2D canvas",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "h264_360p_hw",
			Val: drawOnCanvasParams{
				fileName:    "still-colors-360p.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-360p.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_hw",
			Val: drawOnCanvasParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_480p_hw",
			Val: drawOnCanvasParams{
				fileName:    "still-colors-480p.h264.mp4",
				refFileName: "still-colors-480p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-480p.h264.mp4", "still-colors-480p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_720p_hw",
			Val: drawOnCanvasParams{
				fileName:    "still-colors-720p.h264.mp4",
				refFileName: "still-colors-720p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-720p.h264.mp4", "still-colors-720p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_1080p_hw",
			Val: drawOnCanvasParams{
				fileName:    "still-colors-1080p.h264.mp4",
				refFileName: "still-colors-1080p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-1080p.h264.mp4", "still-colors-1080p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}},
		// TODO(andrescj): add tests for VP8 and VP9.
	})
}

// DrawOnCanvas starts playing a video, draws it on a canvas, and checks a few interesting pixels.
func DrawOnCanvas(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	cr := s.FixtValue().(*chrome.Chrome)
	url := path.Join(server.URL, "video-on-canvas.html")
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", url, err)
	}
	defer conn.Close()

	// Open the reference file to set the canvas size equal to the expected size of the rendered video.
	// This is done in order to prevent scaling filtering artifacts from interfering with our color
	// checks later.
	params := s.Param().(drawOnCanvasParams)
	refPath := s.DataPath(params.refFileName)
	f, err := os.Open(refPath)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", refPath, err)
	}
	defer f.Close()
	refImg, _, err := image.Decode(f)
	if err != nil {
		s.Fatalf("Could not decode %v: %v", refPath, err)
	}
	videoW := refImg.Bounds().Dx()
	videoH := refImg.Bounds().Dy()

	// Note that we set the size of the canvas to 5px more than the video on each dimension.
	// This is so that we can later check that nothing was drawn outside of the expected
	// bounds.
	if err := conn.Call(ctx, nil, "initializeCanvas", videoW+5, videoH+5); err != nil {
		s.Fatal("initializeCanvas() failed: ", err)
	}

	// Now we can play the video and draw it on the canvas.
	if err := conn.Call(ctx, nil, "drawFirstFrameOnCanvas", params.fileName); err != nil {
		s.Fatal("playAndDrawOnCanvas() failed: ", err)
	}

	// Get the contents of the canvas as a PNG image and decode it.
	var canvasPNGB64 string
	if err = conn.Eval(ctx, "getCanvasAsPNG()", &canvasPNGB64); err != nil {
		s.Fatal("getCanvasAsPNG() failed: ", err)
	}
	if !strings.HasPrefix(canvasPNGB64, "data:image/png;base64,") {
		s.Fatal("getCanvasAsPNG() returned data in an unknown format")
	}
	canvasPNG, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(canvasPNGB64, "data:image/png;base64,"))
	if err != nil {
		s.Fatal("Could not base64-decode the data returned by getCanvasAsPNG(): ", err)
	}
	canvasImg, _, err := image.Decode(bytes.NewReader(canvasPNG))
	if err != nil {
		s.Fatal("Could not decode the image returned by getCanvasAsPNG(): ", err)
	}

	// A simple check first: the intrinsic dimensions of the video should match the dimensions of the reference image.
	var intrinsicVideoW, intrinsicVideoH int
	if err = conn.Eval(ctx, "document.getElementById('video').videoWidth", &intrinsicVideoW); err != nil {
		s.Fatal("Could not get the intrinsic video width: ", err)
	}
	if err = conn.Eval(ctx, "document.getElementById('video').videoHeight", &intrinsicVideoH); err != nil {
		s.Fatal("Could not get the intrinsic video height: ", err)
	}
	if intrinsicVideoW != videoW || intrinsicVideoH != videoH {
		s.Fatalf("Unexpected intrinsic dimensions: expected %dx%d; got %dx%d", videoW, videoH, intrinsicVideoW, intrinsicVideoH)
	}

	// Another simple check: nothing should have been drawn at (videoW, videoH).
	c := canvasImg.At(videoW, videoH)
	if play.ColorDistance(color.Black, c) != 0 {
		s.Fatalf("At (%d, %d): expected RGBA = %v; got RGBA = %v", videoW, videoH, color.Black, c)
	}

	// Measurement 1:
	// We'll sample a few interesting pixels and report the color distance with
	// respect to the reference image.
	samples := play.ColorSamplingPointsForStillColorsVideo(videoW, videoH)
	p := perf.NewValues()
	for k, v := range samples {
		expectedColor := refImg.At(v.X, v.Y)
		actualColor := canvasImg.At(v.X, v.Y)
		distance := play.ColorDistance(expectedColor, actualColor)
		// The distance threshold was decided by analyzing the data reported across
		// many devices. Note that:
		//
		// 1) We still report the distances as perf values so we can continue to
		//    analyze and improve.
		// 2) We don't bother to report a total distance if this threshold is
		//    exceeded because it would just make email alerts very noisy.
		if distance > 25 {
			s.Errorf("The color distance for %v = %d exceeds the threshold (25)",
				k, distance)
		}
		if distance != 0 {
			s.Logf("At %v (%d, %d): expected RGBA = %v; got RGBA = %v; distance = %d",
				k, v.X, v.Y, expectedColor, actualColor, distance)
		}
		p.Set(perf.Metric{
			Name:      k,
			Unit:      "None",
			Direction: perf.SmallerIsBetter,
		}, float64(distance))
	}

	if s.HasError() {
		p.Save(s.OutDir())
		return
	}

	// Measurement 2:
	// We report an aggregate distance for the image: we go through all the pixels
	// in the canvas video to add up all the distances and then normalize by the
	// number of pixels at the end.
	totalDistance := 0.0
	for y := 0; y < videoH; y++ {
		for x := 0; x < videoW; x++ {
			expectedColor := refImg.At(x, y)
			actualColor := canvasImg.At(x, y)
			totalDistance += float64(play.ColorDistance(expectedColor, actualColor))
		}
	}
	totalDistance /= float64(videoW * videoH)
	s.Log("The total distance for the entire image is ", totalDistance)
	p.Set(perf.Metric{
		Name:      "total_distance",
		Unit:      "None",
		Direction: perf.SmallerIsBetter,
	}, totalDistance)
	p.Save(s.OutDir())
}
