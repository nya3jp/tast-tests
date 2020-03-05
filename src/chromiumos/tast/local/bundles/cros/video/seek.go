// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

// seekTest is used to describe the config used to run each Seek test.
type seekTest struct {
	filename string // File name to play back.
	numSeeks int    // Amount of times to seek into the <video>.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Seek,
		Desc: "Verifies that seeking works in Chrome, either with or without resolution changes",
		Contacts: []string{
			"acourbot@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"video.html"},
		Params: []testing.Param{{
			Name:              "h264",
			Val:               seekTest{filename: "720_h264.mp4", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "vp8",
			Val:       seekTest{filename: "720_vp8.webm", numSeeks: 25},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"720_vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "vp9",
			Val:       seekTest{filename: "720_vp9.webm", numSeeks: 25},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"720_vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "switch_h264",
			Val:               seekTest{filename: "smpte_bars_resolution_ladder.h264.mp4", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "switch_vp8",
			Val:       seekTest{filename: "smpte_bars_resolution_ladder.vp8.webm", numSeeks: 25},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"smpte_bars_resolution_ladder.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "switch_vp9",
			Val:       seekTest{filename: "smpte_bars_resolution_ladder.vp9.webm", numSeeks: 25},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"smpte_bars_resolution_ladder.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "stress_vp8",
			Val:       seekTest{filename: "720_vp8.webm", numSeeks: 1000},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData: []string{"720_vp8.webm"},
			Timeout:   20 * time.Minute,
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "stress_vp9",
			Val:       seekTest{filename: "720_vp9.webm", numSeeks: 1000},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData: []string{"720_vp9.webm"},
			Timeout:   20 * time.Minute,
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "stress_h264",
			Val:               seekTest{filename: "720_h264.mp4", numSeeks: 1000},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Timeout:           20 * time.Minute,
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_alt",
			Val:               seekTest{filename: "720_h264.mp4", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "vp8_alt",
			Val:               seekTest{filename: "720_vp8.webm", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder"},
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "vp9_alt",
			Val:               seekTest{filename: "720_vp9.webm", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder"},
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "switch_h264_alt",
			Val:               seekTest{filename: "smpte_bars_resolution_ladder.h264.mp4", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "switch_vp8_alt",
			Val:               seekTest{filename: "smpte_bars_resolution_ladder.vp8.webm", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder"},
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "switch_vp9_alt",
			Val:               seekTest{filename: "smpte_bars_resolution_ladder.vp9.webm", numSeeks: 25},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder"},
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "stress_vp8_alt",
			Val:               seekTest{filename: "720_vp8.webm", numSeeks: 1000},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_vp8.webm"},
			Timeout:           20 * time.Minute,
			ExtraSoftwareDeps: []string{"cros_video_decoder"},
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "stress_vp9_alt",
			Val:               seekTest{filename: "720_vp9.webm", numSeeks: 1000},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_vp9.webm"},
			Timeout:           20 * time.Minute,
			ExtraSoftwareDeps: []string{"cros_video_decoder"},
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "stress_h264_alt",
			Val:               seekTest{filename: "720_h264.mp4", numSeeks: 1000},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Timeout:           20 * time.Minute,
			Pre:               pre.ChromeVideoVD(),
		}},
	})
}

// Seek plays a file with Chrome and checks that it can safely be seeked into.
func Seek(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(seekTest)
	if err := play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.filename, testOpt.numSeeks); err != nil {
		s.Fatal("play failed: ", err)
	}
}
