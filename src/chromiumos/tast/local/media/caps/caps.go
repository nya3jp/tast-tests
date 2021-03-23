// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package caps is a package for capabilities used in autotest-capability.
package caps

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/autocaps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// These are constant strings for capabilities in autotest-capability.
// Tests may list these in SoftwareDeps.
// See the below link for detail.
// https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/main/chromeos-base/autotest-capability-default/.
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

	HWDecodeAV1      = Prefix + "hw_dec_av1_1080_30"
	HWDecodeAV1_60   = Prefix + "hw_dec_av1_1080_60"
	HWDecodeAV1_4K   = Prefix + "hw_dec_av1_2160_30"
	HWDecodeAV1_4K60 = Prefix + "hw_dec_av1_2160_60"

	HWDecodeAV1_10BPP      = Prefix + "hw_dec_av1_1080_30_10bpp"
	HWDecodeAV1_60_10BPP   = Prefix + "hw_dec_av1_1080_60_10bpp"
	HWDecodeAV1_4K10BPP    = Prefix + "hw_dec_av1_2160_30_10bpp"
	HWDecodeAV1_4K60_10BPP = Prefix + "hw_dec_av1_2160_60_10bpp"

	HWDecodeHEVC     = Prefix + "hw_dec_hevc_1080_30"
	HWDecodeHEVC60   = Prefix + "hw_dec_hevc_1080_60"
	HWDecodeHEVC4K   = Prefix + "hw_dec_hevc_2160_30"
	HWDecodeHEVC4K60 = Prefix + "hw_dec_hevc_2160_60"

	HWDecodeHEVC10BPP      = Prefix + "hw_dec_hevc_1080_30_10bpp"
	HWDecodeHEVC60_10BPP   = Prefix + "hw_dec_hevc_1080_60_10bpp"
	HWDecodeHEVC4K10BPP    = Prefix + "hw_dec_hevc_2160_30_10bpp"
	HWDecodeHEVC4K60_10BPP = Prefix + "hw_dec_hevc_2160_60_10bpp"

	// Protected Video Decoding
	HWDecodeCBCV1H264 = Prefix + "hw_video_prot_cencv1_h264_cbc"
	HWDecodeCTRV1H264 = Prefix + "hw_video_prot_cencv1_h264_ctr"

	HWDecodeCBCV3AV1 = Prefix + "hw_video_prot_cencv3_av1_cbc"
	HWDecodeCTRV3AV1 = Prefix + "hw_video_prot_cencv3_av1_ctr"

	HWDecodeCBCV3H264 = Prefix + "hw_video_prot_cencv3_h264_cbc"
	HWDecodeCTRV3H264 = Prefix + "hw_video_prot_cencv3_h264_ctr"

	HWDecodeCBCV3HEVC = Prefix + "hw_video_prot_cencv3_hevc_cbc"
	HWDecodeCTRV3HEVC = Prefix + "hw_video_prot_cencv3_hevc_ctr"

	HWDecodeCBCV3VP9 = Prefix + "hw_video_prot_cencv3_vp9_cbc"
	HWDecodeCTRV3VP9 = Prefix + "hw_video_prot_cencv3_vp9_ctr"

	// JPEG Decoding
	HWDecodeJPEG = Prefix + "hw_dec_jpeg"

	// Video Encoding
	HWEncodeH264    = Prefix + "hw_enc_h264_1080_30"
	HWEncodeH264_4K = Prefix + "hw_enc_h264_2160_30"
	// TODO: add here HWEncodeH264OddDimension when video.EncodeAccel has a test
	// exercising odd-dimension encoding.

	HWEncodeVP8             = Prefix + "hw_enc_vp8_1080_30"
	HWEncodeVP8_4K          = Prefix + "hw_enc_vp8_2160_30"
	HWEncodeVP8OddDimension = Prefix + "hw_enc_vp8_odd_dimension"

	HWEncodeVP9             = Prefix + "hw_enc_vp9_1080_30"
	HWEncodeVP9_4K          = Prefix + "hw_enc_vp9_2160_30"
	HWEncodeVP9OddDimension = Prefix + "hw_enc_vp9_odd_dimension"

	// JPEG Encoding
	HWEncodeJPEG = Prefix + "hw_enc_jpeg"

	// Camera
	BuiltinUSBCamera     = Prefix + "builtin_usb_camera"
	BuiltinMIPICamera    = Prefix + "builtin_mipi_camera"
	VividCamera          = Prefix + "vivid_camera"
	BuiltinCamera        = Prefix + "builtin_camera"
	BuiltinOrVividCamera = Prefix + "builtin_or_vivid_camera"
)

// Capability bundles a capability's name and if its optional. The optional
// field allows skipping the verification of a capability and is used on devices
// that technically support e.g. 4K HW decoding, but don't have the static
// autocaps labels set because these devices are so slow that running 4K tests
// would be a huge drain on lab resources.
type Capability struct {
	Name     string // The name of the capability
	Optional bool   // Whether the capability is optional
}

// ErrorReporter is used by VerifyCapabilities() to define a type where only the
// Error reporting method is defined.
type ErrorReporter interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

// VerifyCapabilities compares the capabilities statically defined by the
// autocaps package against those detected by the avtest_label_detect command
// line tool. The function logic follows the table below, essentially verifying
// that a capability is detected if expected and is not detected if not expected
// (either marked as "no" or not statically defined). Capabilities statically
// marked as "disable", or those with Capability.Optional set are not verified.
//
//                 |        Static capability       |
//                 | Yes         | No / Not defined |
//   --------------|-------------|------------------|
//   Detected      | OK          | Fail             |
//   Not detected  | Fail        | OK               |
//
// For more information see:
// /src/third_party/chromiumos-overlay/chromeos-base/autotest-capability-default/files/managed-capabilities.yaml
func VerifyCapabilities(ctx context.Context, e ErrorReporter, avtestLabelToCapability map[string]Capability) error {
	// Get capabilities computed by autocaps package.
	staticCaps, err := autocaps.Read(autocaps.DefaultCapabilityDir, nil)
	if err != nil {
		return errors.Wrap(err, "failed to read statically-set capabilities")
	}
	testing.ContextLog(ctx, "Statically-set capabilities:")
	for c, s := range staticCaps {
		testing.ContextLogf(ctx, "    %v: %v", c, s)
	}

	// Get capabilities detected by "avtest_label_detect" command.
	cmd := testexec.CommandContext(ctx, "avtest_label_detect")
	avOut, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to execute avtest_label_detect")
	}

	detectedLabelRegexp := regexp.MustCompile(`(?m)^Detected label: (.*)$`)
	detectedCaps := make(map[string]struct{})
	for _, m := range detectedLabelRegexp.FindAllStringSubmatch(string(avOut), -1) {
		label := strings.TrimSpace(m[1])
		if c, found := avtestLabelToCapability[label]; found {
			detectedCaps[stripPrefix(c.Name)] = struct{}{}
		}
	}
	testing.ContextLog(ctx, "avtest_label_detect result:")
	for c := range detectedCaps {
		testing.ContextLog(ctx, "    ", c)
	}

	for _, c := range avtestLabelToCapability {
		c.Name = stripPrefix(c.Name)
		state, ok := staticCaps[c.Name]
		if !ok {
			// This is a smoke check: avtestLabelToCapability is using a capability
			// name that is unknown to autocaps.
			e.Errorf("static capabilities don't include %q", c.Name)
			continue
		}

		_, wasDetected := detectedCaps[c.Name]
		switch state {
		case autocaps.Yes:
			if !wasDetected {
				e.Errorf("%q statically set but not detected", c.Name)
			}
		case autocaps.No:
			if wasDetected && !c.Optional {
				e.Errorf("%q detected but not statically set and not optional", c.Name)
			}
		}
	}
	return nil
}

// stripPrefix removes Prefix from the beginning of cap.
func stripPrefix(cap string) string {
	if !strings.HasPrefix(cap, Prefix) {
		panic(fmt.Sprintf("%q doesn't start with %q", cap, Prefix))
	}
	return cap[len(Prefix):]
}
