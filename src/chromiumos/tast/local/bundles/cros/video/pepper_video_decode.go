// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
//	"bytes"
	"context"
//	"encoding/base64"
//	"image"
//	"image/color"
	"net/http"
	"net/http/httptest"
//	"os"
	"path"
//	"strings"
	"time"

//	"chromiumos/tast/common/media/caps"
//	"chromiumos/tast/common/perf"
//	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type pepperVideoDecodeParams struct {
	fileName    string
	refFileName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PepperVideoDecode,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "TODO what this test does",
		Contacts: []string{
			"clarissagarvey@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome", "nacl"},
		Params: []testing.Param{{
			Name: "h264_360p_hw",
			Val: pepperVideoDecodeParams{  // note: currently unused
				fileName:    "still-colors-360p.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"pepper/video_decode.js", "pepper/index.html", "pepper/video_decode.js", "pepper/background.js", "pepper/icon128.png", "pepper/manifest.json", "pepper/pnacl/Release/video_decode.nmf", "pepper/pnacl/Release/video_decode.pexe", "still-colors-360p.h264.mp4", "still-colors-360p.ref.png"},
			Fixture:           "chromeVideo",
		}},
	})
}

// PepperVideoDecode starts playing a video, draws it on a canvas, and checks a few interesting pixels.
func PepperVideoDecode(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	cr := s.FixtValue().(*chrome.Chrome)
	url := path.Join(server.URL, "pepper/index.html")
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", url, err)
	}
	defer conn.Close()

	// My pepper attempt. Not working.
	if err := conn.WaitForExprWithTimeout(ctx, "hasSucceeded", 10*time.Second); err != nil {
		s.Fatal("Did not succeed in message passing within timeout (that is, test failed): ", err)
	}
}
