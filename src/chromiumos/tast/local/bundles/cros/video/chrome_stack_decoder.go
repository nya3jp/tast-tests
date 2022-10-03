// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type chromeStackDecoderTestParam struct {
	dataPath               string
	disableGlobalVaapiLock bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeStackDecoder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies hardware decode acceleration of media::VideoDecoders by running the video_decode_accelerator_tests binary (see go/vd-migration)",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		// TODO(b/185790070): Reenable when MTK8173 (hana, oak, elm) is migrated to the direct VD.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm")),
		Timeout:      4 * time.Minute,
		Fixture:      "graphicsNoChrome",
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_decodeaccel"},
		Params: []testing.Param{{
			Name:              "av1",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         []string{"test-25fps.av1.ivf", "test-25fps.av1.ivf.json"},
		}, {
			Name:              "av1_10bit",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps-10bit.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
			ExtraData:         []string{"test-25fps-10bit.av1.ivf", "test-25fps-10bit.av1.ivf.json"},
		}, {
			Name:              "h264",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "hevc",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.hevc"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraData:         []string{"test-25fps.hevc", "test-25fps.hevc.json"},
		}, {
			Name:              "hevc_10bit",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.hevc10"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC10BPP, "proprietary_codecs"},
			ExtraData:         []string{"test-25fps.hevc10", "test-25fps.hevc10.json"},
		}, {
			Name:              "vp8",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.vp8"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.vp9"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name:              "vp9_2",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.vp9_2"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			ExtraData:         []string{"test-25fps.vp9_2", "test-25fps.vp9_2.json"},
		}, {
			Name:              "av1_resolution_switch",
			Val:               chromeStackDecoderTestParam{dataPath: "resolution_change.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         []string{"resolution_change.av1.ivf", "resolution_change.av1.ivf.json"},
		}, {
			Name:              "h264_resolution_switch",
			Val:               chromeStackDecoderTestParam{dataPath: "switch_1080p_720p_240frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"switch_1080p_720p_240frames.h264", "switch_1080p_720p_240frames.h264.json"},
		}, {
			Name:              "hevc_resolution_switch",
			Val:               chromeStackDecoderTestParam{dataPath: "switch_1080p_720p_240frames.hevc"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraData:         []string{"switch_1080p_720p_240frames.hevc", "switch_1080p_720p_240frames.hevc.json"},
		}, {
			Name:              "vp8_resolution_switch",
			Val:               chromeStackDecoderTestParam{dataPath: "resolution_change_500frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_resolution_switch",
			Val:               chromeStackDecoderTestParam{dataPath: "resolution_change_500frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			// This test uses a video that makes use of the VP9 show-existing-frame feature and is used in Android CTS:
			// https://android.googlesource.com/platform/cts/+/HEAD/tests/tests/media/res/raw/vp90_2_17_show_existing_frame.vp9
			Name:              "vp9_show_existing_frame",
			Val:               chromeStackDecoderTestParam{dataPath: "vda_smoke-vp90_2_17_show_existing_frame.vp9"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"vda_smoke-vp90_2_17_show_existing_frame.vp9", "vda_smoke-vp90_2_17_show_existing_frame.vp9.json"},
		}, {
			// H264 stream in which a profile changes from Baseline to Main.
			Name:              "h264_profile_change",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps_basemain.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"test-25fps_basemain.h264", "test-25fps_basemain.h264.json"},
		}, {
			// Decode VP9 spatial-SVC stream. Precisely the structure in the stream is called k-SVC, where spatial-layers are at key-frame only.
			// The structure is used in Hangouts Meet. go/vp9-svc-hangouts for detail.
			Name:              "vp9_keyframe_spatial_layers",
			Val:               chromeStackDecoderTestParam{dataPath: "keyframe_spatial_layers_180p_360p.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			ExtraData:         []string{"keyframe_spatial_layers_180p_360p.vp9.ivf", "keyframe_spatial_layers_180p_360p.vp9.ivf.json"},
		}, {
			Name:              "av1_odd_dimension",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps-321x241.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         []string{"test-25fps-321x241.av1.ivf", "test-25fps-321x241.av1.ivf.json"},
		}, {
			Name:              "vp8_odd_dimension",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps-321x241.vp8"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps-321x241.vp8", "test-25fps-321x241.vp8.json"},
		}, {
			Name:              "vp9_odd_dimension",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps-321x241.vp9"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps-321x241.vp9", "test-25fps-321x241.vp9.json"},
		}, {
			Name:              "av1_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.av1.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1, "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.av1.ivf", "test-25fps.av1.ivf.json"},
		}, {
			Name:              "h264_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.h264", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "hevc_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.hevc", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.hevc", "test-25fps.hevc.json"},
		}, {
			Name:              "vp8_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.vp8", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "test-25fps.vp9", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "thread_safe_libva_backend"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name:              "av1_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "resolution_change.av1.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1, "thread_safe_libva_backend"},
			ExtraData:         []string{"resolution_change.av1.ivf", "resolution_change.av1.ivf.json"},
		}, {
			Name:              "h264_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "switch_1080p_720p_240frames.h264", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"switch_1080p_720p_240frames.h264", "switch_1080p_720p_240frames.h264.json"},
		}, {
			Name:              "hevc_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "switch_1080p_720p_240frames.hevc", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"switch_1080p_720p_240frames.hevc", "switch_1080p_720p_240frames.hevc.json"},
		}, {
			Name:              "vp8_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "resolution_change_500frames.vp8.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "thread_safe_libva_backend"},
			ExtraData:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_resolution_switch_global_vaapi_lock_disabled",
			Val:               chromeStackDecoderTestParam{dataPath: "resolution_change_500frames.vp9.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "thread_safe_libva_backend"},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}},
	})
}

func ChromeStackDecoder(ctx context.Context, s *testing.State) {
	params := s.Param().(chromeStackDecoderTestParam)

	if err := decoding.RunAccelVideoTest(ctx, s.OutDir(), s.DataPath(params.dataPath), decoding.TestParams{DecoderType: decoding.VD, DisableGlobalVaapiLock: params.disableGlobalVaapiLock}); err != nil {
		s.Fatal("test failed: ", err)
	}
}
