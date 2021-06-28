// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Capability,
		Desc:     "Compare capabilities computed by autocaps package with ones detected by avtest_label_detect",
		Contacts: []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// capabilitiesToVerify is a map of capabilities to verify indexed by the
// avtest_label_detect  capability name.
var capabilitiesToVerify = map[string]caps.Capability{
	"hw_video_acc_h264":             {Name: caps.HWDecodeH264, Optional: false},
	"hw_video_acc_vp8":              {Name: caps.HWDecodeVP8, Optional: false},
	"hw_video_acc_vp9":              {Name: caps.HWDecodeVP9, Optional: false},
	"hw_video_acc_vp9_2":            {Name: caps.HWDecodeVP9_2, Optional: false},
	"hw_video_acc_av1":              {Name: caps.HWDecodeAV1, Optional: false},
	"hw_video_acc_av1_10bpp":        {Name: caps.HWDecodeAV1_10BPP, Optional: false},
	"hw_video_acc_hevc":             {Name: caps.HWDecodeHEVC, Optional: true},
	"hw_video_acc_hevc_10bpp":       {Name: caps.HWDecodeHEVC10BPP, Optional: true},
	"hw_video_acc_h264_4k":          {Name: caps.HWDecodeH264_4K, Optional: true},
	"hw_video_acc_vp8_4k":           {Name: caps.HWDecodeVP8_4K, Optional: true},
	"hw_video_acc_vp9_4k":           {Name: caps.HWDecodeVP9_4K, Optional: true},
	"hw_video_acc_av1_4k":           {Name: caps.HWDecodeAV1_4K, Optional: true},
	"hw_video_acc_av1_4k_10bpp":     {Name: caps.HWDecodeAV1_4K10BPP, Optional: true},
	"hw_video_acc_hevc_4k":          {Name: caps.HWDecodeHEVC4K, Optional: true},
	"hw_video_acc_hevc_4k_10bpp":    {Name: caps.HWDecodeHEVC4K10BPP, Optional: true},
	"hw_jpeg_acc_dec":               {Name: caps.HWDecodeJPEG, Optional: false},
	"hw_video_acc_enc_h264":         {Name: caps.HWEncodeH264, Optional: false},
	"hw_video_acc_enc_vp8":          {Name: caps.HWEncodeVP8, Optional: false},
	"hw_video_acc_enc_vp9":          {Name: caps.HWEncodeVP9, Optional: false},
	"hw_video_acc_enc_h264_4k":      {Name: caps.HWEncodeH264_4K, Optional: false},
	"hw_video_acc_enc_vp8_4k":       {Name: caps.HWEncodeVP8_4K, Optional: false},
	"hw_video_acc_enc_vp9_4k":       {Name: caps.HWEncodeVP9_4K, Optional: false},
	"hw_jpeg_acc_enc":               {Name: caps.HWEncodeJPEG, Optional: false},
	"hw_video_prot_cencv1_h264_cbc": {Name: caps.HWDecodeCBCV1H264, Optional: false},
	"hw_video_prot_cencv1_h264_ctr": {Name: caps.HWDecodeCTRV1H264, Optional: false},
	"hw_video_prot_cencv3_av1_cbc":  {Name: caps.HWDecodeCBCV3AV1, Optional: true},
	"hw_video_prot_cencv3_av1_ctr":  {Name: caps.HWDecodeCTRV3AV1, Optional: true},
	"hw_video_prot_cencv3_h264_cbc": {Name: caps.HWDecodeCBCV3H264, Optional: false},
	"hw_video_prot_cencv3_h264_ctr": {Name: caps.HWDecodeCTRV3H264, Optional: false},
	"hw_video_prot_cencv3_hevc_cbc": {Name: caps.HWDecodeCBCV3HEVC, Optional: false},
	"hw_video_prot_cencv3_hevc_ctr": {Name: caps.HWDecodeCTRV3HEVC, Optional: false},
	"hw_video_prot_cencv3_vp9_cbc":  {Name: caps.HWDecodeCBCV3VP9, Optional: false},
	"hw_video_prot_cencv3_vp9_ctr":  {Name: caps.HWDecodeCTRV3VP9, Optional: false},
}

// Capability compares the static capabilities versus those detected in the DUT.
func Capability(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := caps.VerifyCapabilities(ctx, s, capabilitiesToVerify); err != nil {
		s.Fatal("Test failed: ", err)
	}
}
