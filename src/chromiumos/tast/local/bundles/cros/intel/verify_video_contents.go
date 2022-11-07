// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/hdcputils"
	"chromiumos/tast/local/hdcputils/setup"
	"chromiumos/tast/local/hdcputils/urlconst"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// videoContent struct stores test specific data.
type videoContent struct {
	contentUrls []string
	proxyURL    string
}

func init() {
	// TODO(b/238157101): We are not running this test on any bots intentionally.
	testing.AddTest(&testing.Test{
		Func:         VerifyVideoContents,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies different widevine secure content using shaka player",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInRootfsRemoved",
		HardwareDeps: hwdep.D(setup.PerfHDCPDevices()),
		Params: []testing.Param{{
			Name: "h264",
			Val: videoContent{contentUrls: []string{urlconst.H264SD, urlconst.H264HD, urlconst.H264UHD, urlconst.H264Fullsample, urlconst.H264CBCS},
				proxyURL: urlconst.ProxyURL},
			Timeout: 9 * time.Minute,
		}, {
			Name: "vp9",
			Val: videoContent{contentUrls: []string{urlconst.VP9Subsample, urlconst.VP9Superframe},
				proxyURL: urlconst.ProxyURL},
			Timeout: 4 * time.Minute,
		}, {
			Name: "hevc",
			Val: videoContent{contentUrls: []string{urlconst.HEVCclip,
				urlconst.HEVCCBCS, urlconst.HEVC4K, urlconst.HEVCclipSD, urlconst.HEVCclipHD},
				proxyURL: urlconst.ProxyURL},
			Timeout: 9 * time.Minute,
		}},
	})
}

func VerifyVideoContents(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	testData := s.Param().(videoContent)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(cleanupCtx)

	const expectedAudioNode = "INTERNAL_SPEAKER"
	for _, contentURL := range testData.contentUrls {
		videoConn, err := hdcputils.LaunchShakaPlayer(ctx, cr, contentURL, testData.proxyURL)
		if err != nil {
			s.Fatal("Failed to launch shaka player: ", err)
		}

		// Select HW_SECURE_ALL in video robustness from the gear icon.
		const videoRobustnessName = "HW_SECURE_ALL"
		if err := videoConn.SelectVideoRobustness(ctx, videoRobustnessName); err != nil {
			s.Fatal("Failed to select HW_SECURE_ALL: ", err)
		}

		// Now click on the green play button.
		// Check if audio is coming while video playback.
		if err := videoConn.PlayVideo(ctx); err != nil {
			s.Fatal("Failed to play video: ", err)
		}
		if err := videoConn.VerifyVideoPlayWithDuration(ctx, 11, cras, expectedAudioNode); err != nil {
			s.Fatal("Failed to play verify video or audio routing: ", err)
		}

		// Switch video between full screen and default screen 5 times.
		const iterValue = 5
		if err := videoConn.FullScreenEntryExit(ctx, iterValue); err != nil {
			s.Fatal("Failed to switch video fillscreen: ", err)
		}

		// Take screenshot and check video area is blank to confirm HW DRM is working.
		const isExtDisplay = false
		if err := videoConn.VerifyVideoBlankScreen(ctx, s.OutDir(), isExtDisplay); err != nil {
			s.Fatal("Failed to verify video blank screen: ", err)
		}
		videoConn.Conn.CloseTarget(ctx)
	}
}
