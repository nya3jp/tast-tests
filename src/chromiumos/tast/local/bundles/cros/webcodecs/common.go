// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"chromiumos/tast/local/media/videotype"
)

type HardwareAcceleration string

const (
	preferHardware HardwareAcceleration = "prefer-hardware"
	preferSoftware HardwareAcceleration = "prefer-software"
)

// Convert videotype.Codec to codec in MIME type.
// See https://developer.mozilla.org/en-US/docs/Web/Media/Formats/codecs_parameter for detail.
func ToMIMECodec(codec videotype.Codec) string {
	switch codec {
	case videotype.H264:
		// H.264 Baseline Level 3.1.
		return "avc1.42001E"
	case videotype.VP8:
		return "vp8"
	case videotype.VP9:
		// VP9 profile 0 level 1.0 8-bit depth.
		return "vp09.00.10.08"
	}
	return ""
}
