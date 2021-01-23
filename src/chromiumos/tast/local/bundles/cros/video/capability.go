// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/media/caps"
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
// avtest_label_detect command line tool name.
var capabilitiesToVerify = map[string]caps.Capability{
	"hw_video_acc_h264":        {caps.HWDecodeH264, false},
	"hw_video_acc_vp8":         {caps.HWDecodeVP8, false},
	"hw_video_acc_vp9":         {caps.HWDecodeVP9, false},
	"hw_video_acc_vp9_2":       {caps.HWDecodeVP9_2, false},
	"hw_video_acc_h264_4k":     {caps.HWDecodeH264_4K, true},
	"hw_video_acc_vp8_4k":      {caps.HWDecodeVP8_4K, true},
	"hw_video_acc_vp9_4k":      {caps.HWDecodeVP9_4K, true},
	"hw_jpeg_acc_dec":          {caps.HWDecodeJPEG, false},
	"hw_video_acc_enc_h264":    {caps.HWEncodeH264, false},
	"hw_video_acc_enc_vp8":     {caps.HWEncodeVP8, false},
	"hw_video_acc_enc_vp9":     {caps.HWEncodeVP9, false},
	"hw_video_acc_enc_h264_4k": {caps.HWEncodeH264_4K, false},
	"hw_video_acc_enc_vp8_4k":  {caps.HWEncodeVP8_4K, false},
	"hw_video_acc_enc_vp9_4k":  {caps.HWEncodeVP9_4K, false},
	"hw_jpeg_acc_enc":          {caps.HWEncodeJPEG, false},
	"builtin_usb_camera":       {caps.BuiltinUSBCamera, false},
	"builtin_mipi_camera":      {caps.BuiltinMIPICamera, false},
	"vivid_camera":             {caps.VividCamera, false},
	"builtin_camera":           {caps.BuiltinCamera, false},
	"builtin_or_vivid_camera":  {caps.BuiltinOrVividCamera, false},
}

// Capability compares the static capabilities versus those detected in the DUT.
func Capability(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := caps.VerifyCapabilities(ctx, capabilitiesToVerify); err != nil {
		s.Fatal("Test failed: ", err)
	}
}
