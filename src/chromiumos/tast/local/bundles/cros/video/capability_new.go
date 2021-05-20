// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/newcaps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CapabilityNew,
		Desc:     "Compare capabilities computed by mediacaps package with ones detected by media_capabilities",
		Contacts: []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// capabilitiesToVerify is a map of capabilities to verify indexed by the
// media_capabilities capability name.
var capabilitiesToVerify = map[string]string{
	"hw_dec_h264_baseline_1080":  newcaps.HWDecodeH264Baseline,
	"hw_dec_h264_baseline_2160":  newcaps.HWDecodeH264Baseline_4K,
	"hw_dec_h264_main_1080":      newcaps.HWDecodeH264Main,
	"hw_dec_h264_main_2160":      newcaps.HWDecodeH264Main_4K,
	"hw_dec_h264_high_1080":      newcaps.HWDecodeH264High,
	"hw_dec_h264_high_2160":      newcaps.HWDecodeH264High_4K,
	"hw_dec_vp8_1080":            newcaps.HWDecodeHVP8,
	"hw_dec_vp8_2160":            newcaps.HWDecodeHVP8_4K,
	"hw_dec_vp9_1080":            newcaps.HWDecodeHVP9Profile0,
	"hw_dec_vp9_2160":            newcaps.HWDecodeHVP9Profile0_4K,
	"hw_dec_vp9_2_1080":          newcaps.HWDecodeHVP9Profile2,
	"hw_dec_vp9_2_2160":          newcaps.HWDecodeHVP9Profile2_4K,
	"hw_dec_av1_main_1080":       newcaps.HWDecodeAV1Main,
	"hw_dec_av1_main_2160":       newcaps.HWDecodeAV1Main_4K,
	"hw_dec_av1_main_10bpp_1080": newcaps.HWDecodeAV1Main10BPP,
	"hw_dec_av1_main_10bpp_2160": newcaps.HWDecodeAV1Main10BPP_4K,
	"hw_dec_h265_main_1080":      newcaps.HWDecodeH265Main,
	"hw_dec_h265_main_2160":      newcaps.HWDecodeH265Main_4K,
	// TODO: Add HW Protected Video Decoding.
	"hw_enc_h264_baseline_1080": newcaps.HWEncodeH264Baseline,
	"hw_enc_h264_baseline_2160": newcaps.HWEncodeH264Baseline_4K,
	"hw_enc_h264_main_1080":     newcaps.HWEncodeH264Main,
	"hw_enc_h264_main_2160":     newcaps.HWEncodeH264Main_4K,
	"hw_enc_h264_high_1080":     newcaps.HWEncodeH264High,
	"hw_enc_h264_high_2160":     newcaps.HWEncodeH264High_4K,
	"hw_enc_vp8_1080":           newcaps.HWEncodeHVP8,
	"hw_enc_vp8_2160":           newcaps.HWEncodeHVP8_4K,
	"hw_enc_vp9_1080":           newcaps.HWEncodeHVP9Profile0,
	"hw_enc_vp9_2160":           newcaps.HWEncodeHVP9Profile0_4K,
	"hw_dec_jpeg":               newcaps.HWDecodeJPEG,
	"hw_enc_jpeg":               newcaps.HWEncodeJPEG,

	// Move to camera.CapabilityNew
	"builtin_usb_camera":      newcaps.BuiltinUSBCamera,
	"builtin_mipi_camera":     newcaps.BuiltinMIPICamera,
	"vivid_camera":            newcaps.VividCamera,
	"builtin_camera":          newcaps.BuiltinCamera,
	"builtin_or_vivid_camera": newcaps.BuiltinOrVividCamera,
}

func CapabilityNew(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := newcaps.VeirifyCapabilities(ctx, s, capabilitiesToVerify); err != nil {
		s.Fatal("Test failed: ", err)
	}
}
