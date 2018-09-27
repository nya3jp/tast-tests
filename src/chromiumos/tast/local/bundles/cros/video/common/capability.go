// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

const (
	capPrefix = "autotest-capability:"

	// Video Decoding
	H264HWDecoding     = capPrefix + "hw_dec_h264_1080_30"
	H264HWDecoding60   = capPrefix + "hw_dec_h264_1080_60"
	H264HWDecoding4K   = capPrefix + "hw_dec_h264_2160_30"
	H264HWDecoding4K60 = capPrefix + "hw_dec_h264_2160_60"

	VP8HWDecoding     = capPrefix + "hw_dec_vp8_1080_30"
	VP8HWDecoding60   = capPrefix + "hw_dec_vp8_1080_60"
	VP8HWDecoding4K   = capPrefix + "hw_dec_vp8_2160_30"
	VP8HWDecoding4K60 = capPrefix + "hw_dec_vp8_2160_60"

	VP9HWDecoding     = capPrefix + "hw_dec_vp9_1080_30"
	VP9HWDecoding60   = capPrefix + "hw_dec_vp9_1080_60"
	VP9HWDecoding4K   = capPrefix + "hw_dec_vp9_2160_30"
	VP9HWDecoding4K60 = capPrefix + "hw_dec_vp9_2160_60"

	VP9_2HWDecoding     = capPrefix + "hw_dec_vp9-2_1080_30"
	VP9_2HWDecoding60   = capPrefix + "hw_dec_vp9-2_1080_60"
	VP9_2HWDecoding4K   = capPrefix + "hw_dec_vp9-2_2160_30"
	VP9_2HWDecoding4K60 = capPrefix + "hw_dec_vp9-2_2160_60"

	// Jpeg Decoding
	JPEGHWDecoding = capPrefix + "hw_dec_jpeg"

	// Video Encoding
	H264HWEncoding = capPrefix + "hw_enc_h264_1080_30"
	VP8HWEncoding  = capPrefix + "hw_enc_vp8_1080_30"
	VP9HWEncoding  = capPrefix + "hw_enc_vp9_1080_30"

	// Jpeg Encoding
	JPEGHWEncoding = capPrefix + "hw_enc_jpeg"

	// Camera
	USBCamera = capPrefix + "usb_camera"
)
