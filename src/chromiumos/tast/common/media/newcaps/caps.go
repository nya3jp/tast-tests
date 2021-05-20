// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package capsnew is a package for capabilities used in autotest-capability.
package capsnew

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/mediacaps"
	"chromiumos/tast/testing"
)

const (
	Prefix = "media-capability:"

	// HW Video Decoding.
	HWDecodeH264Baseline    = Prefix + "hw_dec_h264_baseline_1080"
	HWDecodeH264Baseline_4K = Prefix + "hw_dec_h264_baseline_2160"
	HWDecodeH264Main        = Prefix + "hw_dec_h264_main_1080"
	HWDecodeH264Main_4K     = Prefix + "hw_dec_h264_main_2160"
	HWDecodeH264High        = Prefix + "hw_dec_h264_high_1080"
	HWDecodeH264High_4K     = Prefix + "hw_dec_h264_high_2160"

	HWDecodeHVP8    = Prefix + "hw_dec_vp8_1080"
	HWDecodeHVP8_4K = Prefix + "hw_dec_vp8_2160"

	HWDecodeHVP9Profile0    = Prefix + "hw_dec_vp9_1080"
	HWDecodeHVP9Profile0_4K = Prefix + "hw_dec_vp9_2160"
	HWDecodeHVP9Profile2    = Prefix + "hw_dec_vp9_2_1080"
	HWDecodeHVP9Profile2_4K = Prefix + "hw_dec_vp9_2_2160"

	HWDecodeAV1Main         = Prefix + "hw_dec_av1_main_1080"
	HWDecodeAV1Main_4K      = Prefix + "hw_dec_av1_main_2160"
	HWDecodeAV1Main10BPP    = Prefix + "hw_dec_av1_main_10bpp_1080"
	HWDecodeAV1Main10BPP_4K = Prefix + "hw_dec_av1_main_10bpp_2160"

	HWDecodeH264Baseline    = Prefix + "hw_dec_h264_baseline_1080"
	HWDecodeH264Baseline_4K = Prefix + "hw_dec_h264_baseline_2160"
	HWDecodeH264Main        = Prefix + "hw_dec_h264_main_1080"
	HWDecodeH264Main_4K     = Prefix + "hw_dec_h264_main_2160"
	HWDecodeH264High        = Prefix + "hw_dec_h264_high_1080"
	HWDecodeH264High_4K     = Prefix + "hw_dec_h264_high_2160"

	// TODO: Add HW Protected Video Decoding.

	// HW Video Encoding.
	HWEncodeH264Baseline    = Prefix + "hw_enc_h264_baseline_1080"
	HWEncodeH264Baseline_4K = Prefix + "hw_enc_h264_baseline_2160"
	HWEncodeH264Main        = Prefix + "hw_enc_h264_main_1080"
	HWEncodeH264Main_4K     = Prefix + "hw_enc_h264_main_2160"
	HWEncodeH264High        = Prefix + "hw_enc_h264_high_1080"
	HWEncodeH264High_4K     = Prefix + "hw_enc_h264_high_2160"
	HWEncodeHVP8            = Prefix + "hw_enc_vp8_1080"
	HWEncodeHVP8_4K         = Prefix + "hw_enc_vp8_2160"
	HWEncodeHVP9Profile0    = Prefix + "hw_enc_vp9_1080"
	HWEncodeHVP9Profile0_4K = Prefix + "hw_enc_vp9_2160"

	// HW JPEG Decoding.
	HWDecodeJPEG = Prefix + "hw_dec_jpeg"

	// HW JPEG Encoding.
	HWEncodeJPEG = Prefix + "hw_enc_jpeg"

	// Camera.
	BuiltinUSBCamera     = Prefix + "builtin_usb_camera"
	BuiltinMIPICamera    = Prefix + "builtin_mipi_camera"
	VividCamera          = Prefix + "vivid_camera"
	BuiltinCamera        = Prefix + "builtin_camera"
	BuiltinOrVividCamera = Prefix + "builtin_or_vivid_camera"
)

func VerifyCapabilities(ctx context.Context, s *testing.State, capabilitiesToVerify map[string]Capability) error {
	// Get capabilities computed by mediacaps package.
	staticCaps, err := mediacpas.Read(mediacaps.DefaultCapabilitiesDir, nil)
	if err != nil {
		return errors.Wrap(err, "failed to read statically-set capabilities")
	}
	testing.ContextLog(ctx, "Statically-set capabilities:")
	for c, s := range staticCaps {
		testing.ContextLogf(ctx, "    %v: %v", c, s)
	}

	// Get capabilities detected by "media_capabilities" command.
	cmd := testexec.CommandContext(ctx, "media_capabilities")
	mcOut, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to execute avtest_label_detect")
	}

	detectedCaps := make(map[string]struct{})

	return nil
}

// stripPrefix removes Prefix from the beginning of cap.
func stripPrefix(cap string) string {
	if !strings.HasPrefix(cap, Prefix) {
		panic(fmt.Sprintf("%q doesn't start with %q", cap, Prefix))
	}
	return cap[len(Prefix):]
}
