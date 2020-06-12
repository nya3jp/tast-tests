// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

// TODO(andrescj): test other codecs.
func init() {
	testing.AddTest(&testing.Test{
		Func: DrawOnCanvas,
		Desc: "Checks that a video can be drawn on a 2D canvas",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "h264_360p",
			Val:               "still-colors-360p.h264.mp4",
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-360p.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_720p",
			Val:               "still-colors-720p.h264.mp4",
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-720p.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_1080p",
			Val:               "still-colors-1080p.h264.mp4",
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraData:         []string{"video-on-canvas.html", "still-colors-1080p.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}},
	})
}

func DrawOnCanvas(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	cr := s.PreValue().(*chrome.Chrome)
	url := server.URL + "/video-on-canvas.html"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", url, err)
	}
	if err := conn.EvalPromise(ctx, fmt.Sprintf("playAndDrawOnCanvas(%q)", s.Param().(string)), nil); err != nil {
		s.Fatal(err)
	}
}
