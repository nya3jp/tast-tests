// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/local/mtbf/debug"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF021PlayGSMMSHEAACLCAAC,
		Desc:         "Checks GSM_MS, HE-AAC, LC-AAC formats audio playback in Chrome is working",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
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

	audioplayer, mtbferr := audio.NewPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audioplayer.Close(ctx)

	s.Logf("Play %s", audioFile)
	if mtbferr := audioplayer.StartToPlay(ctx, audioFile, s.DataPath(audioFile)); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.IsPlaying(ctx, time.Second*3); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	s.Logf("Pause %s", audioFile)
	if mtbferr := audioplayer.Pause(ctx); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.IsPausing(ctx, time.Second*3); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	s.Logf("Play %s", audioFile)
	if mtbferr := audioplayer.Play(ctx); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.IsPlaying(ctx, time.Second*3); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, time.Second*3)
}
