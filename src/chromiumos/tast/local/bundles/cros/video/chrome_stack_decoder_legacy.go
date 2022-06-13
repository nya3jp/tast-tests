// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// cqAllowlist is the list of stable device models we want to enable CQ for.
// Note: The CQ runs a test pre-commit dozens/hundreds of times per post-commit release build.
// Adding tests to the CQ is therefore extremely expensive. As tests in the CQ may prevent Chrome
// from upreving, only devices that are present in Chromium CQ are added. Consider carefully which
// tests/devices to add to the CQ.
var cqAllowlist = []string{
	"eve",
	"kevin",
}

type chromeStackDecoderLegacyTestParam struct {
	dataPath               string
	disableGlobalVaapiLock bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeStackDecoderLegacy,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies hardware decode acceleration by running the video_decode_accelerator_tests binary with the legacy implementation",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "graphicsNoChrome",
		Params: []testing.Param{{
			Name:              "h264",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps.h264"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			// Run H264 video decode tests on CQ, limited to devices on the CQ allow list.
			Name:              "h264_cq",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps.h264"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(cqAllowlist...)),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "h264_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps.h264", disableGlobalVaapiLock: true},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "vp8",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps.vp8"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp8_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps.vp8", disableGlobalVaapiLock: true},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported", "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps.vp9"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name:              "vp9_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps.vp9", disableGlobalVaapiLock: true},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported", "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name:              "h264_resolution_switch",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "switch_1080p_720p_240frames.h264"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			ExtraData:         []string{"switch_1080p_720p_240frames.h264", "switch_1080p_720p_240frames.h264.json"},
		}, {
			Name:              "h264_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "switch_1080p_720p_240frames.h264", disableGlobalVaapiLock: true},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"switch_1080p_720p_240frames.h264", "switch_1080p_720p_240frames.h264.json"},
		}, {
			Name:              "vp8_resolution_switch",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "resolution_change_500frames.vp8.ivf"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			ExtraData:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "resolution_change_500frames.vp8.ivf", disableGlobalVaapiLock: true},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported", "thread_safe_libva_backend"},
			ExtraData:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_resolution_switch",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "resolution_change_500frames.vp9.ivf"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "resolution_change_500frames.vp9.ivf", disableGlobalVaapiLock: true},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported", "thread_safe_libva_backend"},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			Name: "vp8_odd_dimensions",
			Val:  chromeStackDecoderLegacyTestParam{dataPath: "test-25fps-321x241.vp8"},
			// TODO(b/138915749): Enable once decoding odd dimension videos is fixed.
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			ExtraData:         []string{"test-25fps-321x241.vp8", "test-25fps-321x241.vp8.json"},
		}, {
			Name: "vp9_odd_dimensions",
			Val:  chromeStackDecoderLegacyTestParam{dataPath: "test-25fps-321x241.vp9"},
			// TODO(b/138915749): Enable once decoding odd dimension videos is fixed.
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			ExtraData:         []string{"test-25fps-321x241.vp9", "test-25fps-321x241.vp9.json"},
		}, {
			// This test uses a video that makes use of the VP9 show-existing-frame feature and is used in Android CTS:
			// https://android.googlesource.com/platform/cts/+/HEAD/tests/tests/media/res/raw/vp90_2_17_show_existing_frame.vp9
			Name:              "vp9_show_existing_frame",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "vda_smoke-vp90_2_17_show_existing_frame.vp9"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			ExtraData:         []string{"vda_smoke-vp90_2_17_show_existing_frame.vp9", "vda_smoke-vp90_2_17_show_existing_frame.vp9.json"},
		}, {
			// H264 stream in which a profile changes from Baseline to Main.
			Name:              "h264_profile_change",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "test-25fps_basemain.h264"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			ExtraData:         []string{"test-25fps_basemain.h264", "test-25fps_basemain.h264.json"},
		}, {
			// Run with HW decoder using VA-API only because only the HW decoder can decode SVC stream correctly today.
			// Decode VP9 spatial-SVC stream. Precisely the structure in the stream is called k-SVC, where spatial-layers are at key-frame only.
			// The structure is used in Hangouts Meet. go/vp9-svc-hangouts for detail.
			Name:              "vp9_keyframe_spatial_layers",
			Val:               chromeStackDecoderLegacyTestParam{dataPath: "keyframe_spatial_layers_180p_360p.vp9.ivf"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "vaapi", "video_decoder_legacy_supported"},
			ExtraData:         []string{"keyframe_spatial_layers_180p_360p.vp9.ivf", "keyframe_spatial_layers_180p_360p.vp9.ivf.json"},
		}},
	})
}

func ChromeStackDecoderLegacy(ctx context.Context, s *testing.State) {
	param := s.Param().(chromeStackDecoderLegacyTestParam)

	if err := decoding.RunAccelVideoTest(ctx, s.OutDir(), s.DataPath(param.dataPath), decoding.TestParams{DecoderType: decoding.VDA, DisableGlobalVaapiLock: param.disableGlobalVaapiLock}); err != nil {
		s.Fatal("test failed: ", err)
	}
}
