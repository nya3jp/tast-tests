// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package caps is a package for capabilities used in autotest-capability.
package caps

// These are constant strings for capabilities in autotest-capability.
// Tests may list these in SoftwareDeps.
// See the below link for detail.
// https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/master/chromeos-base/autotest-capability-default/.
const (
	// Prefix is the prefix of capability.
	Prefix = "autotest-capability:"

	// Video Decoding
	HWDecodeH264      = Prefix + "hw_dec_h264_1080_30"
	HWDecodeH264_60   = Prefix + "hw_dec_h264_1080_60"
	HWDecodeH264_4K   = Prefix + "hw_dec_h264_2160_30"
	HWDecodeH264_4K60 = Prefix + "hw_dec_h264_2160_60"

	HWDecodeVP8      = Prefix + "hw_dec_vp8_1080_30"
	HWDecodeVP8_60   = Prefix + "hw_dec_vp8_1080_60"
	HWDecodeVP8_4K   = Prefix + "hw_dec_vp8_2160_30"
	HWDecodeVP8_4K60 = Prefix + "hw_dec_vp8_2160_60"

	HWDecodeVP9      = Prefix + "hw_dec_vp9_1080_30"
	HWDecodeVP9_60   = Prefix + "hw_dec_vp9_1080_60"
	HWDecodeVP9_4K   = Prefix + "hw_dec_vp9_2160_30"
	HWDecodeVP9_4K60 = Prefix + "hw_dec_vp9_2160_60"

	HWDecodeVP9_2      = Prefix + "hw_dec_vp9-2_1080_30"
	HWDecodeVP9_2_60   = Prefix + "hw_dec_vp9-2_1080_60"
	HWDecodeVP9_2_4K   = Prefix + "hw_dec_vp9-2_2160_30"
	HWDecodeVP9_2_4K60 = Prefix + "hw_dec_vp9-2_2160_60"

	// JPEG Decoding
	HWDecodeJPEG = Prefix + "hw_dec_jpeg"

	// Video Encoding
	HWEncodeH264              = Prefix + "hw_enc_h264_1080_30"
	HWEncodeH264_4K           = Prefix + "hw_enc_h264_2160_30"
	// TODO: add here HWEncodeH264_odd_dimension when video.EncodeAccel has a test
	// exercising odd-dimension encoding.

	HWEncodeVP8               = Prefix + "hw_enc_vp8_1080_30"
	HWEncodeVP8_4K            = Prefix + "hw_enc_vp8_2160_30"
	HWEncodeVP8_odd_dimension = Prefix + "hw_enc_vp8_odd_dimension"

	HWEncodeVP9               = Prefix + "hw_enc_vp9_1080_30"
	HWEncodeVP9_4K            = Prefix + "hw_enc_vp9_2160_30"
	HWEncodeVP9_odd_dimension = Prefix + "hw_enc_vp9_odd_dimension"


	// JPEG Encoding
	HWEncodeJPEG = Prefix + "hw_enc_jpeg"

	// Camera
	BuiltinUSBCamera     = Prefix + "builtin_usb_camera"
	BuiltinMIPICamera    = Prefix + "builtin_mipi_camera"
	VividCamera          = Prefix + "vivid_camera"
	BuiltinCamera        = Prefix + "builtin_camera"
	BuiltinOrVividCamera = Prefix + "builtin_or_vivid_camera"
)
