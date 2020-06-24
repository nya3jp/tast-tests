// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

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
			Name:              "h264_360p_hw",
			Val:               "still-colors-360p.h264.mp4",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-360p.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_360p_exotic_crop_hw",
			Val:               "still-colors-720x480-cropped-to-640x360.h264.mp4",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-720x480-cropped-to-640x360.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_480p_hw",
			Val:               "still-colors-480p.h264.mp4",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-480p.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_720p_hw",
			Val:               "still-colors-720p.h264.mp4",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-720p.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_1080p_hw",
			Val:               "still-colors-1080p.h264.mp4",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-1080p.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}},
		// TODO(andrescj): add tests for VP8 and VP9.
	})
}

// DrawOnCanvas starts playing a video, draws it on a canvas, and checks a few interesting pixels.
func DrawOnCanvas(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	cr := s.PreValue().(*chrome.Chrome)
	url := path.Join(server.URL, "video-on-canvas.html")
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", url, err)
	}
	defer conn.Close()
	if err := conn.Eval(ctx, fmt.Sprintf("playAndDrawOnCanvas(%q)", s.Param().(string)), nil); err != nil {
		s.Fatal("playAndDrawOnCanvas() failed: ", err)
	}
}
