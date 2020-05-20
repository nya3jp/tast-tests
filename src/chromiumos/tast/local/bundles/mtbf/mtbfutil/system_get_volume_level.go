// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbfutil

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemGetVolumeLevel,
		Desc:         "Get system sound volume level",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
	})
}

// SystemGetVolumeLevel get system volume by invoks chrome.audio.getDevice.
func SystemGetVolumeLevel(ctx context.Context, s *testing.State) {
	var cr = s.PreValue().(*chrome.Chrome)
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer conn.Close()

	s.Log("Starting to get system sound level")
	volume, err := audio.GetOSVolume(ctx, conn)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioGetVolLvl, err))
	}
	s.Logf("Get system sound level %d", volume)
}
