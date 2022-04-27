// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/features"
	camperf "chromiumos/tast/local/camera/perf"
	"chromiumos/tast/local/camera/testpage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

// The features we want to test.
const (
	hdrnet        string = "hdrnet"
	gcamAE               = "gcam_ae"
	faceDetection        = "face_detection"
)

// The override file path for the features we want to test.
const (
	hdrnetConfigOverridePath        string = "/run/camera/hdrnet_config.json"
	gcamAEConfigOverridePath               = "/run/camera/gcam_ae_config.json"
	faceDetectionConfigOverridePath        = "/run/camera/face_detection_config.json"
)

// The feature controls that we want to override in the test.
const (
	hdrnetEnable        string = "hdrnet_enable"
	gcamAEEnable               = "gcam_ae_enable"
	faceDetectionEnable        = "face_detection_enable"
)

var featureDesc = map[string]struct {
	deviceConfig     features.FeatureConfig
	overrideFilePath string
}{
	hdrnet:        {nil, hdrnetConfigOverridePath},
	gcamAE:        {nil, gcamAEConfigOverridePath},
	faceDetection: {nil, faceDetectionConfigOverridePath},
}

type featureOverride map[string]features.FeatureConfig

func init() {
	testing.AddTest(&testing.Test{
		Func:         HDRnetPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Runs the HDRnet performance tests",
		Contacts:     []string{"jcliang@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"camera_feature_hdrnet", "chrome", caps.BuiltinMIPICamera},
		Data:         []string{"camera_page.html", "camera_page.js"},
		Vars: []string{
			// Test only the specified resolution (360p, 720p, 1080p).
			"resolution",
			// Overrides the default measure duration (in seconds).
			"duration",
			// Comma separated list of profilers to run (cpu, gpu, perf_record, top).
			"profilers",
		},
		Timeout: 60 * time.Minute,
	})
}

func HDRnetPerf(ctx context.Context, s *testing.State) {
	model, err := crosconfig.Get(ctx, "/", "name")
	if err != nil {
		s.Errorf("Failed to get device model: %s", err)
	}
	modelConf, err := features.NewModelConfig(model)
	if err != nil {
		s.Errorf("Failed to get feature profile for device model %s: %v", model, err)
	}

	testing.ContextLogf(ctx, "Model config: %s", modelConf)

	// Load the device-specific feature config as base.
	for t, d := range featureDesc {
		d.deviceConfig = features.NewFeatureConfig()
		err := modelConf.FeatureConfig(t, d.deviceConfig, nil)
		if err != nil {
			s.Errorf("Failed to get feature config for %s: %v", t, err)
		}
		s.Logf("%s config: %s", t, d.deviceConfig)

	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ExtraArgs(
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream",
	))
	if err != nil {
		s.Error("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Common resolutions for video use-cases.
	resolutions := []struct {
		name   string
		width  int
		height int
	}{
		{"360p", 640, 360},
		{"720p", 1280, 720},
		{"1080p", 1920, 1080},
	}

	testCases := []struct {
		name     string
		override featureOverride
	}{
		// All features disabled.
		{"baseline", featureOverride{
			hdrnet:        {hdrnetEnable: false},
			gcamAE:        {gcamAEEnable: false},
			faceDetection: {faceDetectionEnable: false},
		}},
		// All features enabled.
		{"all_on", featureOverride{
			hdrnet:        {hdrnetEnable: true},
			gcamAE:        {gcamAEEnable: true},
			faceDetection: {faceDetectionEnable: true},
		}},
		{"HDRnet_only", featureOverride{
			hdrnet:        {hdrnetEnable: true},
			gcamAE:        {gcamAEEnable: false},
			faceDetection: {faceDetectionEnable: false},
		}},
		{"GcamAE_only", featureOverride{
			hdrnet:        {hdrnetEnable: false},
			gcamAE:        {gcamAEEnable: true},
			faceDetection: {faceDetectionEnable: false},
		}},
		{"FaceDetection_only", featureOverride{
			hdrnet:        {hdrnetEnable: false},
			gcamAE:        {gcamAEEnable: false},
			faceDetection: {faceDetectionEnable: true},
		}},
	}

	const (
		defaultSubtestTimeout  time.Duration = 5 * time.Minute
		defaultStableDuration                = 5 * time.Second
		defaultMeasureDuration               = 3 * time.Minute
	)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait CPU to become idle: ", err)
	}

	subtestTimeout := defaultSubtestTimeout
	measureDuration := defaultMeasureDuration
	if dur, hasDuration := s.Var("duration"); hasDuration {
		v, err := strconv.ParseInt(dur, 10, 64)
		if err != nil {
			s.Fatal("Invalid measure duration: ", err)
		}
		measureDuration = time.Duration(v) * time.Second
		subtestTimeout = measureDuration + defaultStableDuration + 30*time.Second
	}

	var profList []string
	if plist, hasProfilers := s.Var("profilers"); hasProfilers {
		profList = strings.Split(plist, ",")
	}

	pv := perf.NewValues()
	for _, r := range resolutions {
		if res, hasResolution := s.Var("resolution"); hasResolution {
			if r.name != res {
				continue
			}
		}

		for _, t := range testCases {
			subTestCtx, cancel := context.WithTimeout(ctx, subtestTimeout)
			name := fmt.Sprintf("%s-%s", r.name, t.name)
			s.Run(subTestCtx, name, func(cts context.Context, s *testing.State) {
				// Override controls must be set before we open the camera device.
				if err := overrideFeatureConfigs(&t.override); err != nil {
					s.Fatal("Failed to override feature config")
				}

				page := testpage.New(server.URL)
				cst := testpage.NewConstraints(r.width, r.height, testpage.UserFacing)
				if err := page.OpenWithConstraints(subTestCtx, cr, cst); err != nil {
					s.Fatal("Failed to open camera test page: ", err)
				}

				// We need a subfolder for the perf and top data of each subtest.
				subtestOutdir := filepath.Join(s.OutDir(), name)
				if err := os.Mkdir(subtestOutdir, 0755); err != nil {
					s.Fatal("Failed to create subtest output directory")
				}
				pctx := camperf.Start(subTestCtx, defaultStableDuration, measureDuration, name, subtestOutdir, profList)
				if pctx == nil {
					s.Fatal("Failed to start profilers")
				}
				if err := pctx.Wait(); err != nil {
					s.Fatal("Failed to collect performance profiling data")
				}
				pv.Merge(pctx.Results)

				if err := page.Close(subTestCtx); err != nil {
					s.Error("Failed to close camera test page: ", err)
				}
				removeOverrideFiles(&t.override)
			})
			cancel()
		}
	}
	pv.Save(s.OutDir())
}

// overrideFeatureConfigs overrides the feature controls specified in |o|.
func overrideFeatureConfigs(o *featureOverride) error {
	removeOverrideFiles(o)
	for f, c := range *o {
		out := features.MeldFeatureConfig(featureDesc[f].deviceConfig, c)
		if err := features.WriteFeatureConfig(out, featureDesc[f].overrideFilePath, true); err != nil {
			return errors.Wrapf(err, "failed to override feature config of %s", f)
		}
	}
	return nil
}

// removeOverrideFiles removes the override files for the features in |o|.
func removeOverrideFiles(o *featureOverride) {
	for f := range *o {
		os.Remove(featureDesc[f].overrideFilePath)
	}
}
