// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path"

	"chromiumos/tast/common/media/caps"
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

	// Open the reference file to set the canvas size equal to the expected size of the rendered video. This
	// is done in order to prevent scaling filtering artifacts from interfering with our color checks later.
	refPath := s.DataPath(s.Param().(drawOnCanvasParams).refFileName)
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
	if err := conn.Call(ctx, nil, "initializeCanvas", videoW, videoH); err != nil {
		s.Fatal("initializeCanvas() failed: ", err)
	}

	// Now we can play the video and draw it on the canvas.
	if err := conn.Call(ctx, nil, "playAndDrawOnCanvas", s.Param().(drawOnCanvasParams).fileName); err != nil {
		s.Fatal("playAndDrawOnCanvas() failed: ", err)
	}
}
