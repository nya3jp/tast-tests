// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/local/mtbf/debug"
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

	// Start to play.
	if mtbferr := audio.PlayFromDownloadsFolder(ctx, files, s.DataPath(audioFile), audioFile); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audio.ClickButton(ctx, tconn, "Close")
	s.Logf("Play %s", audioFile)
	if err := files.WaitForElement(ctx, filesapp.RoleRootWebArea, "Audio Player", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferrors.New(mtbferrors.ChromeOpenAudioPlayer, err))
	}
	testing.Sleep(ctx, time.Second)
	if mtbferr := audio.IsPlaying(ctx, tconn, 3*time.Second); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	s.Logf("Pause %s", audioFile)
	if err := files.WaitForElement(ctx, "button", "Pause", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferrors.New(mtbferrors.AudioWaitPauseButton, err))
	}
	if mtbferr := audio.Pause(ctx, tconn); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)
	if mtbferr := audio.IsPausing(ctx, tconn, 3*time.Second); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	s.Logf("Play %s", audioFile)
	if err := files.WaitForElement(ctx, "button", "Play", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferrors.New(mtbferrors.AudioWaitPlayButton, err))
	}
	if mtbferr := audio.Play(ctx, tconn); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)
	if mtbferr := audio.IsPlaying(ctx, tconn, 3*time.Second); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 3*time.Second)
}
