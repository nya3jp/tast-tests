// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

// These are constant strings for capabilities in autotest-capability.
// See the below link for detail.
// https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/master/chromeos-base/autotest-capability-default/
const (
	capPrefix = "autotest-capability:"

	// Video Decoding
	HWDecodeH264      = capPrefix + "hw_dec_h264_1080_30"
	HWDecodeH264_60   = capPrefix + "hw_dec_h264_1080_60"
	HWDecodeH264_4K   = capPrefix + "hw_dec_h264_2160_30"
	HWDecodeH264_4K60 = capPrefix + "hw_dec_h264_2160_60"

	HWDecodeVP8      = capPrefix + "hw_dec_vp8_1080_30"
	HWDecodeVP8_60   = capPrefix + "hw_dec_vp8_1080_60"
	HWDecodeVP8_4K   = capPrefix + "hw_dec_vp8_2160_30"
	HWDecodeVP8_4K60 = capPrefix + "hw_dec_vp8_2160_60"

	HWDecodeVP9      = capPrefix + "hw_dec_vp9_1080_30"
	HWDecodeVP9_60   = capPrefix + "hw_dec_vp9_1080_60"
	HWDecodeVP9_4K   = capPrefix + "hw_dec_vp9_2160_30"
	HWDecodeVP9_4K60 = capPrefix + "hw_dec_vp9_2160_60"

	HWDecodeVP9_2      = capPrefix + "hw_dec_vp9-2_1080_30"
	HWDecodeVP9_2_60   = capPrefix + "hw_dec_vp9-2_1080_60"
	HWDecodeVP9_2_4K   = capPrefix + "hw_dec_vp9-2_2160_30"
	HWDecodeVP9_2_4K60 = capPrefix + "hw_dec_vp9-2_2160_60"

	// JPEG Decoding
	HWDecodeJPEG = capPrefix + "hw_dec_jpeg"

	// Video Encoding
	HWEncodeH264 = capPrefix + "hw_enc_h264_1080_30"
	HWEncodeVP8  = capPrefix + "hw_enc_vp8_1080_30"
	HWEncodeVP9  = capPrefix + "hw_enc_vp9_1080_30"

	// JPEG Encoding
	HWEncodeJPEG = capPrefix + "hw_enc_jpeg"

	// Camera
	USBCamera = capPrefix + "usb_camera"
)
