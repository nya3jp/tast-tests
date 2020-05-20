// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbfutil

import (
	"context"
	"strconv"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemChangeVolume,
		Desc:         "Changes system volume by givin variable",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"mtbfutil.SystemChangeVolume.var"},
	})
}

// SystemChangeVolume adjust volume to variables by invoking chrome.audio.setProperties.
func SystemChangeVolume(ctx context.Context, s *testing.State) {
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

	v := s.RequiredVar("mtbfutil.SystemChangeVolume.var")
	volume, err := strconv.Atoi(v)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioInputVol, err, v))
	}

	s.Logf("Starting test volume change to %d%%", volume)
	if mbtferr := audio.CheckOSVolume(ctx, conn, volume); err != nil {
		s.Fatal(mbtferr)
	}
}
