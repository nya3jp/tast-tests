// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

const decoderKey = "kVideoDecoderName"
const decoderValue = `"MojoVideoDecoder"`

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF005VideoHWDecoding,
		Desc:         "HWDecodingResolutionChanging(MTBF005): Check what codecs are supported on your device under test at go/croscodecmatrix, includes H264, VP8, VP9",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Params: []testing.Param{{
			Name: "h264",
			Val:  "http://crosvideo.appspot.com/?codec=mp4&amp;cycle=true",
		}, {
			Name: "vp8",
			Val:  "http://crosvideo.appspot.com/?codec=vp8&amp;cycle=true",
		}, {
			Name: "vp9",
			Val:  "http://crosvideo.appspot.com/?codec=vp9&amp;cycle=true",
		}},
	})
}

func MTBF005VideoHWDecoding(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	url := s.Param().(string)

	s.Log("Access chrome://media-internals")
	mediaInterURL := "chrome://media-internals"

	connInter, mtbferr := mtbfchrome.NewConn(ctx, cr, mediaInterURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer connInter.Close()
	defer connInter.CloseTarget(ctx)

	s.Log("Clear previous video/audio logs")
	clrExpr := `document.querySelector("hide-players-button).click()`
	connInter.Eval(ctx, clrExpr, nil)

	connVideo, mtbferr := mtbfchrome.NewConn(ctx, cr, url)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer connVideo.Close()
	defer connVideo.CloseTarget(ctx)

	testing.Sleep(ctx, time.Second*4)

	s.Log("Open video log")
	openLog := `document.querySelector("#player-list li:last-child label").click()`
	connInter.Eval(ctx, openLog, nil)

	s.Log("Search decoder type")
	decoder := ""
	checkProp := fmt.Sprintf(`
		new Promise((resolve, reject) => {
			var rowList = document.querySelectorAll("#player-property-table>tbody>tr");
			for (row of rowList) {
			  if (row.querySelector("td").innerHTML === %q) {
				resolve(row.querySelector("td:nth-of-type(2)").innerHTML);
			  }
			}
		})
	`, decoderKey)
	connInter.EvalPromise(ctx, checkProp, &decoder)

	if decoder != decoderValue {
		s.Fatal(mtbferrors.New(mtbferrors.VideoDecoder, nil, decoder, decoderValue))
	}
}
