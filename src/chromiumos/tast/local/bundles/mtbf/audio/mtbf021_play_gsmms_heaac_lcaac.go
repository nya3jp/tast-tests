// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/audio/player"
	"chromiumos/tast/local/chrome"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF021PlayGSMMSHEAACLCAAC,
		Desc:         "Checks GSM_MS, HE-AAC, LC-AAC formats audio playback in Chrome is working",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline"},
		Data:         []string{"intelinsideA_gsm_ms.wav", "AAC-LC_v4_Stereo_48000Hz.m4a", "HE-AAC_Stereo_32000Hz.m4a"},
		Params: []testing.Param{{
			Name: "gsmms",
			Val:  "intelinsideA_gsm_ms.wav",
		}, {
			Name: "lcaac",
			Val:  "AAC-LC_v4_Stereo_48000Hz.m4a",
		}, {
			Name: "heaac",
			Val:  "HE-AAC_Stereo_32000Hz.m4a",
		}},
	})
}

// MTBF021PlayGSMMSHEAACLCAAC plays a given file with Chrome.
func MTBF021PlayGSMMSHEAACLCAAC(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	audioFile := s.Param().(string)

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()
	defer tconn.CloseTarget(ctx)

	s.Log("Open the Files App")
	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer filesapp.Close(ctx, tconn)

	audioplayer, mtbferr := player.New(s, audioFile, s.DataPath(audioFile))
	if mtbferr != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioPlayer, mtbferr))
	}

	// Verify play
	if mtbferr := audioplayer.StartToPlay(ctx, files); mtbferr != nil {
		s.Error(mtbferr)
	}
	defer player.ClickButton(ctx, tconn, "Close")
	s.Logf("Play %s", audioFile)
	if err := files.WaitForElement(ctx, filesapp.RoleRootWebArea, "Audio Player", time.Minute); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeOpenAudioPlayer, err))
	}
	if err := player.IsPlaying(ctx, tconn, 3*time.Second); err != nil {
		s.Error(mtbferrors.New(mtbferrors.AudioPlayPause, err))
	}

	s.Logf("Pause %s", audioFile)
	if err := files.WaitForElement(ctx, "button", "Pause", time.Minute); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioWaitPauseButton, err))
	}
	if err := player.Pause(ctx, tconn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.AudioClickPauseButton, err))
	}
	if err := player.IsPausing(ctx, tconn, 3*time.Second); err != nil {
		s.Error(mtbferrors.New(mtbferrors.AudioPlayPause, err))
	}
	s.Logf("Play %s", audioFile)
	if err := files.WaitForElement(ctx, "button", "Play", time.Minute); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioWaitPlayButton, err))
	}
	if err := player.Play(ctx, tconn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.AudioClickPlayButton, err))
	}
	if err := player.IsPlaying(ctx, tconn, 3*time.Second); err != nil {
		s.Error(mtbferrors.New(mtbferrors.AudioPlayPause, err))
	}

	testing.Sleep(ctx, 3*time.Second)
}
