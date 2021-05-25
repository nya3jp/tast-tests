// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
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

	// Now we can play the video and draw it on the canvas without any scaling.
	if err := conn.Call(ctx, nil, "playAndDrawOnCanvas", params.fileName, videoW, videoH); err != nil {
		s.Fatal("playAndDrawOnCanvas() failed: ", err)
	}

	// getCanvasColorAt allows us to get the color of the canvas at an arbitrary point (x, y).
	getCanvasColorAt := func(x, y int) (color.Color, error) {
		var c struct {
			Components map[int]byte `json:"data"`
		}
		if err = conn.Call(ctx, &c, "getCanvasColorAt", x, y); err != nil {
			return nil, errors.Wrap(err, "getCanvasColorAt failed")
		}
		return color.RGBA{c.Components[0], c.Components[1], c.Components[2], c.Components[3]}, nil
	}

	// A simple check first: nothing should have been drawn at (videoW, videoH).
	c, err := getCanvasColorAt(videoW, videoH)
	if err != nil {
		s.Fatalf("Could not get canvas color at (%d, %d): %v", videoW, videoH, err)
	}
	nothingColor := color.RGBA{0, 0, 0, 0}
	if c != nothingColor {
		s.Fatalf("At (%d, %d): expected RGBA = %v; got RGBA = %v", videoW, videoH, nothingColor, c)
	}

	// Measurement 1:
	// We'll sample a few interesting pixels and report the color distance with
	// respect to the reference image.
	samples := play.ColorSamplingPointsForStillColorsVideo(videoW, videoH)
	p := perf.NewValues()
	for k, v := range samples {
		expectedColor := refImg.At(v.X, v.Y)
		actualColor, err := getCanvasColorAt(v.X, v.Y)
		if err != nil {
			s.Fatalf("Could not get canvas color at (%d, %d): %v", v.X, v.Y, err)
		}
		distance := play.ColorDistance(expectedColor, actualColor)
		s.Logf("At %v (%d, %d): expected RGBA = %v; got RGBA = %v; distance = %d",
			k, v.X, v.Y, expectedColor, actualColor, distance)
		p.Set(perf.Metric{
			Name:      k,
			Unit:      "None",
			Direction: perf.SmallerIsBetter,
		}, float64(distance))
	}

	// Measurement 2:
	// We report an aggregate distance for the image: we go through all the pixels
	// in the canvas video to add up all the distances and then normalize by the
	// number of pixels at the end.
	totalDistance := 0.0
	for y := 0; y < videoH; y++ {
		// We get one row at a time because querying one pixel at a time takes too long.
		var row struct {
			Data map[int]byte `json:"data"`
		}
		if err = conn.Call(ctx, &row, "getRowData", y, videoW); err != nil {
			s.Fatal("getRowData() failed: ", err)
		}

		for x := 0; x < videoW; x++ {
			expectedColor := refImg.At(x, y)
			aR := row.Data[4*x]
			aG := row.Data[4*x+1]
			aB := row.Data[4*x+2]
			aA := row.Data[4*x+3]
			actualColor := color.RGBA{aR, aG, aB, aA}
			if err != nil {
				s.Fatalf("Could not get canvas color at (%d, %d): %v", x, y, err)
			}
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
