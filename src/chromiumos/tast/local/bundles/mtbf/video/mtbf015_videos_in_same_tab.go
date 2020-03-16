// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF015VideosInSameTab,
		Desc:         "VideosInSameTab(MTBF015): Play multiple MPEG4 format videos simultaneously in same tab",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "cros_video_decoder"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Data:         []string{"101587703.mp4", "112068856.mp4", "113102615.mp4", "world.mp4", "mpeg.html"},
	})
}

var (
	videoSelectors = []string{
		"body > table:nth-child(2) > tbody > tr > td:nth-child(1) > video",
		"body > table:nth-child(2) > tbody > tr > td:nth-child(2) > video",
		"body > table:nth-child(3) > tbody > tr > td:nth-child(1) > video",
		"body > table:nth-child(3) > tbody > tr > td:nth-child(2) > video",
	}
)

func MTBF015VideosInSameTab(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	mpegURL := server.URL + "/mpeg.html"
	conn, err := mtbfchrome.NewConn(ctx, cr, mpegURL)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	failed := false
	for idx, selector := range videoSelectors {
		if err := media.IsPlaying(ctx, conn, time.Second*5, selector); err != nil {
			if err := dom.PlayElement(ctx, conn, selector); err != nil {
				s.Fatal(mtbferrors.New(mtbferrors.VideoNoPlay, err, fmt.Sprintf("Video %d", idx+1)))
			}
			failed = true
		}
	}

	if !failed {
		return
	}

	for _, selector := range videoSelectors {
		if err := media.IsPlaying(ctx, conn, time.Second*5, selector); err != nil {
			s.Fatal("MTBF failed: ", err)
		}
	}
}
