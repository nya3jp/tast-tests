// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/libvda"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LibvdaDecode,
		Desc:         "Checks that video decoding using libvda's Mojo connection to GAVDA is working",
		Contacts:     []string{"alexlau@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Params: []testing.Param{{
			Name: "h264",
			Val:  "h264",
			// "chrome_internal" is needed because H.264 is a proprietary codec
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraData:         []string{"test-25fps.h264"},
		}, {
			Name:              "vp8",
			Val:               "vp8",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8"},
		}, {
			Name:              "vp9",
			Val:               "vp9",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9"},
		}},
	})
}

func LibvdaDecode(ctx context.Context, s *testing.State) {
	logFileName := "output_libvda_" + s.Param().(string) + ".txt"
	videoFile := "test-25fps." + s.Param().(string)
	libvda.RunGPUFileDecodeTest(ctx, s, logFileName, videoFile)
}
