// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF009AdjustVolume,
		Desc:         "Adjust volume by key input",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
	})
}

// MTBF009AdjustVolume adjust volume by key input.
func MTBF009AdjustVolume(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer tconn.Close()
	defer tconn.CloseTarget(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Set sound volume to 50")
	audio.SetOSVolume(ctx, tconn, 50)
	testing.Sleep(ctx, 3*time.Second)

	s.Log("Set sound unmute")
	testing.Sleep(ctx, 3*time.Second)
	audio.SetOSVolumeMute(ctx, tconn, false)
	testing.Sleep(ctx, 3*time.Second)

	pressKeyAndVerify(ctx, tconn, s, kb, "F10")
	pressKeyAndVerify(ctx, tconn, s, kb, "F9")
	testing.Sleep(ctx, 3*time.Second)

	pressKey(ctx, s, kb, "F8")
	s.Log("Verify operation system output audio device is mute")
	isMute, mtbferr := audio.IsOSVolumeMute(ctx, tconn)
	if err != nil {
		s.Fatal(mtbferr)
	}
	if !isMute {
		s.Fatal(mtbferrors.New(mtbferrors.AudioMute, nil))
	}
	audio.SetOSVolumeMute(ctx, tconn, false)
}

// pressKey do a key press.
func pressKey(ctx context.Context, s *testing.State, kb *input.KeyboardEventWriter, key string) (err error) {
	s.Logf("Press %s to change volume level", key)
	if err = kb.Accel(ctx, key); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, key)
	}
	return nil
}

// pressKeyAndVerify do a key press and verify it's result.
func pressKeyAndVerify(ctx context.Context, tconn *chrome.Conn, s *testing.State, kb *input.KeyboardEventWriter, key string) (err error) {
	volume, mbtferr := audio.GetOSVolume(ctx, tconn)
	if err != nil {
		s.Fatal(mbtferr)
	}
	s.Logf("Get current OS volume(%d)", volume)
	pressKey(ctx, s, kb, key)
	testing.Sleep(ctx, 3*time.Second)
	newvolume, mbtferr := audio.GetOSVolume(ctx, tconn)
	if err != nil {
		s.Fatal(mbtferr)
	}
	s.Log("Verify operation system output aduio device has changed sound level")
	i := newvolume - volume
	if i < 0 {
		i = i * -1
	}
	if i != 4 {
		s.Fatal(mtbferrors.New(mtbferrors.AudioChgVol, nil, newvolume, volume))
	}

	return
}
