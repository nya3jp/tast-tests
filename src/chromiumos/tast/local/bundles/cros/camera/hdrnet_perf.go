// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/features"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

const (
	hdrnet        string = "hdrnet"
	gcamAE               = "gcam_ae"
	faceDetection        = "face_detection"
)

const (
	hdrnetConfigOverridePath        string = "/run/camera/hdrnet_config.json"
	gcamAEConfigOverridePath               = "/run/camera/gcam_ae_config.json"
	faceDetectionConfigOverridePath        = "/run/camera/face_detection_config.json"
)

type hdrnetConfig struct {
	Enable                bool    `json:"hdrnet_enable"`
	DumpBuffer            bool    `json:"dump_buffer"`
	LogFrameMetadata      bool    `json:"log_frame_metadata"`
	HDRRatio              float32 `json:"hdr_ratio"`
	SpatialFilterSigma    float32 `json:"spatial_filter_sigma"`
	RangeFilterSigma      float32 `json:"range_filter_sigma"`
	IIRFilterStrength     float32 `json:"iir_filter_strength"`
	MaxGainBlendThreshold float32 `json:"max_gain_blend_threshold"`
}

// AE stats input mode
const (
	fromVendorAEStats int = 0
	fromYUVImage          = 1
)

// AE override mode
const (
	withExposureCompensation int = 0
	withManualSensorControl      = 1
	withVendorTag                = 2
)

type gcamAEConfig struct {
	Enable                       bool               `json:"gcam_ae_enable"`
	AEFrameInterval              int                `json:"ae_frame_interval"`
	MaxHDRRatio                  map[string]float32 `json:"max_hdr_ratio"`
	BaseExpComp                  float32            `json:"exp_comp"`
	AEStatsInputMode             int                `json:"ae_stats_input_mode"`
	AEOverrideMode               int                `json:"ae_override_mode"`
	LogFrameMetadata             bool               `json:"log_frame_metadata"`
	TETTargetThreshold           float32            `json:"tet_target_threshold_log2"`
	TETConvergeThreshold         float32            `json:"tet_converge_threshold_log2"`
	TETConvergeStabilizeDuration float32            `json:"tet_converge_stabilize_duration_ms"`
	TETRescanThreshold           float32            `json:"tet_rescan_threshold_log2"`
	SmallStep                    float32            `json:"small_step_log2"`
	LargeStep                    float32            `json:"large_step_log2"`
	LogSceneBrightnessThreshold  float32            `json:"log_scene_brightness_threshold"`
	DefaultTETRetentionDuration  int                `json:"tet_retention_duration_ms_default"`
	FaceTETRetentionDurtaion     int                `json:"tet_retention_duration_ms_with_face"`
	InitialTET                   float32            `json:"initial_tet"`
	InitialHDRRatio              float32            `json:"initial_hdr_ratio"`
	HDRRatioStep                 float32            `json:"hdr_ratio_step"`
}

type faceDetectionConfig struct {
	Enable           bool `json:"face_detection_enable"`
	FrameInterval    int  `json:"fd_frame_interval"`
	LogFrameMetadata bool `json:"log_frame_metadata"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         HDRnetPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Runs the HDRnet performance tests",
		Contacts:     []string{"jcliang@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"camera_app", "camera_feature_hdrnet", "chrome", caps.BuiltinMIPICamera},
		Fixture:      "ccaTestBridgeReady",
		Timeout:      6 * time.Minute,
	})
}

// newHDRnetConfig returns an initialized HDRnetConfig suitable for testing
// purposes.
func newHDRnetConfig() *hdrnetConfig {
	config := new(hdrnetConfig)
	config.HDRRatio = 1.0
	config.IIRFilterStrength = 0.7
	config.SpatialFilterSigma = 1.5
	return config
}

// newGcamAEConfig returns an initialized GcamAEConfig suitable for testing
// purposes.
func newGcamAEConfig() *gcamAEConfig {
	config := new(gcamAEConfig)
	config.AEFrameInterval = 2
	config.MaxHDRRatio = map[string]float32{
		"1":  5.0,
		"2":  5.0,
		"4":  5.0,
		"8":  4.0,
		"16": 2.0,
		"32": 1.1,
	}
	config.AEStatsInputMode = fromVendorAEStats
	config.AEOverrideMode = withManualSensorControl
	config.SmallStep = 0.1
	config.LargeStep = 0.5
	config.LogSceneBrightnessThreshold = 1.5
	config.TETTargetThreshold = 0.1
	config.TETConvergeStabilizeDuration = 500
	config.TETConvergeThreshold = 0.1
	config.TETRescanThreshold = 0.2
	config.DefaultTETRetentionDuration = 1000
	config.FaceTETRetentionDurtaion = 3000
	config.InitialTET = 33.33
	config.InitialHDRRatio = 1.0
	config.HDRRatioStep = 0.2
	return config
}

// newFaceDetectionConfig returns an initialized FaceDetectionConfig suitable
// for testing purposes.
func newFaceDetectionConfig() *faceDetectionConfig {
	config := new(faceDetectionConfig)
	config.FrameInterval = 10
	return config
}

func HDRnetPerf(ctx context.Context, s *testing.State) {
	model, err := crosconfig.Get(ctx, "/", "name")
	if err != nil {
		s.Errorf("Failed to get device model: %s", err)
	}
	modelConf, err := features.New(model, nil)
	if err != nil {
		s.Errorf("Failed to get feature profile for device model %s: %s", model, err)
	}

	testing.ContextLogf(ctx, "Model config: %s", modelConf)

	var featureDesc = map[string]struct {
		config           interface{}
		overrideFilePath string
	}{
		hdrnet:        {newHDRnetConfig(), hdrnetConfigOverridePath},
		gcamAE:        {newGcamAEConfig(), gcamAEConfigOverridePath},
		faceDetection: {newFaceDetectionConfig(), faceDetectionConfigOverridePath},
	}

	for t, d := range featureDesc {
		err := features.GetFeatureConfig(modelConf, t, d.config, nil)
		if err != nil {
			s.Errorf("Failed to get feature config for %s: %s", t, err)
		}
		testing.ContextLogf(ctx, "%s config: %s", t, d.config)

		features.SetOverrideFeatureConfig(d.config, d.overrideFilePath)
	}

	runSubTest := s.FixtValue().(cca.FixtureData).RunTestWithApp
	cr := s.FixtValue().(cca.FixtureData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to establish connection to the test API extension")
	}
	subTestTimeout := 120 * time.Second

	for _, t := range []struct {
		name       string
		resolution cca.Resolution
	}{
		{"360p", cca.Resolution{Width: 640, Height: 360}},
		{"720p", cca.Resolution{Width: 1280, Height: 720}},
		{"1080p", cca.Resolution{Width: 1920, Height: 1080}},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, t.name, func(ctx context.Context, s *testing.State) {
			if err := runSubTest(ctx, func(ctx context.Context, app *cca.App) error {
				return testCameraPreview(ctx, app, t.resolution, tconn)
			}, cca.TestWithAppParams{StopAppOnlyIfExist: true}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", t.name, err)
			}
		})
		cancel()
	}
}

func testCameraPreview(ctx context.Context, app *cca.App, rs cca.Resolution, tconn *chrome.TestConn) error {
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to Photo mode")
	}

	app.IterateResolutions(ctx, cca.VideoResolution, cca.FacingFront, map[cca.Resolution]bool{rs: true}, func(r cca.Resolution) error {
		if r != rs {
			return nil
		}
		testing.ContextLogf(ctx, "Testing resolution %s", rs)
		testing.Sleep(ctx, 30*time.Second)
		return nil
	})

	if err := app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close Chrome Camera App")
	}
	return nil
}
