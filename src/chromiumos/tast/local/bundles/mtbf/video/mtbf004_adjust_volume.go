// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF004AdjustVolume,
		Desc:         "Adjust volume to 100% by invoking chrome.audio.setProperties",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoginReuse(),
		Params: []testing.Param{{
			Name: "10",
			Val:  10,
		}, {
			Name: "100",
			Val:  100,
		}},
	})
}

func MTBF004AdjustVolume(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err = dom.WaitForDocumentReady(ctx, conn); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoDocLoad, err))
	}
	s.Log("Document is ready")

	volume := s.Param().(int)
	s.Logf("Starting test volume change to %d%%", volume)
	if mtbferr := audio.CheckOSVolume(ctx, conn, volume); err != nil {
		s.Fatal(mtbferr)
	}
}
