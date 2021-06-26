// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hal3

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	builtInUSBCameraConfigPath = "/etc/camera/camera_characteristics.conf"
	cameraHALGlobPattern       = "/usr/lib*/camera_hal/*.so"
	jsonConfigPath             = "/var/cache/camera/test_config.json"
)

// mediaSettings is used to unmarshal media profile in ARC.
type mediaSettings struct {
	XMLName           xml.Name `xml:"MediaSettings"`
	CamcorderProfiles []struct {
		CameraID       int `xml:"cameraId,attr"`
		EncoderProfile []struct {
			Video struct {
				Width     int `xml:"width,attr"`
				Height    int `xml:"height,attr"`
				FrameRate int `xml:"frameRate,attr"`
			}
		}
	}
}

// ArcCameraUID returns the user id used by camera service and camera test binary.
func ArcCameraUID() (int, error) {
	uid, err := sysutil.GetUID("arc-camera")
	if err != nil {
		return -1, errors.Wrap(err, "failed to get uid of arc-camera")
	}
	return int(uid), nil
}

// IsV1Legacy returns true if the built-in camera is not qualified for Android
// Camera HALv3 LIMITED hardware level.
func IsV1Legacy(ctx context.Context) (bool, error) {
	// For unibuild, we can determine if a device is v1 legacy by checking
	// 'legacy-usb' under path '/camera' in cros_config.
	legacyUSB, err := crosconfig.Get(ctx, "/camera", "legacy-usb")
	if err != nil && !crosconfig.IsNotFound(err) {
		return false, errors.Wrap(err, "failed to get legacy-usb state from cros_config")
	}
	if legacyUSB == "true" {
		return true, nil
	}

	// For non-unibuild, we can check if 'v1device' presents in the config file
	// '/etc/camera/camera_chracteristics.conf'.
	config, err := ioutil.ReadFile("/etc/camera/camera_characteristics.conf")
	if os.IsNotExist(err) {
		// The device does not have camera_characteristics.conf, which might
		// because:
		//   (1) It has USB built-in camera(s) but it does not have ARC.
		//   (2) It only has MIPI built-in camera(s).
		//   (3) It has no built-in camera on it. (e.g. Chromebox)
		//
		// For these cases, only (2) should run LIMITED level tests.
		//
		// TODO(b/163436311): Use cros_config to distinguish these cases after
		// we are able to get counts for USB/MIPI cameras respectively.
		cameraHALPaths, err := filepath.Glob(cameraHALGlobPattern)
		if err != nil {
			return false, errors.Wrap(err, "failed to get camera HALs paths")
		}

		for _, path := range cameraHALPaths {
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if name != "usb" && name != "ip" {
				return false, nil
			}
		}
		return true, nil
	} else if err != nil {
		return false, errors.Wrap(err, "failed to read camera_characteristics.conf")
	}
	return strings.Contains(string(config), "v1device"), nil
}

// getRecordingParams gets the recording parameters from the media profile in
// ARC, which would be used as an argument of cros_camera_test.
func getRecordingParams(ctx context.Context) (string, error) {
	mediaProfilePath, err := readOrGenerateMediaProfilePath(ctx)
	if err != nil {
		return "", err
	}
	out, err := ioutil.ReadFile(mediaProfilePath)
	if err != nil {
		return "", err
	}
	var settings mediaSettings
	if err := xml.Unmarshal(out, &settings); err != nil {
		return "", err
	}

	var supportConstantFrameRate int
	isV1, err := IsV1Legacy(ctx)
	if err != nil {
		return "", err
	}
	if !isV1 {
		supportConstantFrameRate = 1
	}

	seen := make(map[string]struct{})
	var params []string
	for _, cprof := range settings.CamcorderProfiles {
		for _, eprof := range cprof.EncoderProfile {
			video := eprof.Video
			param := fmt.Sprintf("%d:%d:%d:%d:%d", cprof.CameraID,
				video.Width, video.Height, video.FrameRate, supportConstantFrameRate)
			if _, ok := seen[param]; !ok {
				seen[param] = struct{}{}
				params = append(params, param)
			}
		}
	}
	return strings.Join(params, ","), nil
}

// readOrGenerateMediaProfilePath returns the ARC media profile file path, and possibly generates the file if not exists.
func readOrGenerateMediaProfilePath(ctx context.Context) (string, error) {
	if path, err := crosconfig.Get(ctx, "/arc/media-profiles", "system-path"); err == nil {
		return path, nil
	} else if !crosconfig.IsNotFound(err) {
		return "", err
	}
	if err := testexec.CommandContext(ctx, "generate_camera_profile").Run(testexec.DumpLogOnError); err != nil {
		return "", err
	}
	return "/var/cache/camera/media_profiles.xml", nil
}

// crosCameraTestConfig is the config for running cros_camera_test.
// Note that cameraHALPath and cameraFacing are mutually exclusive, see
// GetCmdLineTestCameraFacing() and InitializeTest() in [1] for more details.
// [1] https://chromium.git.corp.google.com/chromiumos/platform2/+/363b9b16d6d16937743e619526d51ab59970caf6/camera/camera3_test/camera3_module_test.cc?pli=1#1239
type crosCameraTestConfig struct {
	cameraHALPath        string // path to the camera HAL to test
	cameraFacing         string // facing of the camera to test, such as "front" or "back".
	gtestFilter          string // filter for Google Test
	recordingParams      string // resolutions and fps to test in recording
	perfLog              string // path to the performance log
	portraitModeTestData string // test data for portrait mode test.
}

// toArgs converts crosCameraTestConfig to a list of argument strings.
func (t *crosCameraTestConfig) toArgs() []string {
	// Make the 3A timeout longer since test lab is in a dark environment.
	args := []string{"--3a_timeout_multiplier=2"}
	if t.cameraHALPath != "" {
		args = append(args, "--camera_hal_path="+t.cameraHALPath)
	}
	if t.cameraFacing != "" {
		args = append(args, "--camera_facing="+t.cameraFacing)
	}
	if t.recordingParams != "" {
		args = append(args, "--recording_params="+t.recordingParams)
	}
	if t.perfLog != "" {
		// TODO(shik): Change the test binary to use --perf_log.
		args = append(args, "--output_log="+t.perfLog)
	}
	if t.portraitModeTestData != "" {
		args = append(args, "--portrait_mode_test_data="+t.portraitModeTestData)
	}
	return args
}

func runBasicCrosCameraTest(ctx context.Context, test *gtest.GTest) error {

	args, err := test.Args()
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Running ", shutil.EscapeSlice(args))

	cmd := testexec.CommandContext(ctx, args[0], args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, string(out))
	}

	return nil
}

// runCrosCameraTest runs cros_camera_test with the arguments generated from the
// config.  The cros-camera service must be stopped before calling this function.
func runCrosCameraTest(ctx context.Context, cfg crosCameraTestConfig) error {
	// The test is performance sensitive and frame drops might cause test failures.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		// TODO(b/169122360): Find a better way to reduce the flakiness.
		// Currently it's too flaky so we cannot return an error here.
		testing.ContextLog(ctx, "Failed to wait for CPU to become idle: ", err)
	}

	uid, err := ArcCameraUID()
	if err != nil {
		return err
	}

	// Run a initialization test before the complete test
	t := gtest.New("cros_camera_test",
		gtest.Filter("Camera3ModuleFixture.NumberOfCameras:Camera3ModuleFixture.OpenDevice"),
		gtest.ExtraArgs(cfg.toArgs()...),
		gtest.UID(uid))

	if err := runBasicCrosCameraTest(ctx, t); err != nil {
		return err
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("missing out dir")
	}

	t = gtest.New("cros_camera_test",
		gtest.TempLogfile(filepath.Join(outDir, "cros_camera_test_*.log")),
		gtest.Filter(cfg.gtestFilter),
		gtest.ExtraArgs(cfg.toArgs()...),
		gtest.UID(uid))

	if args, err := t.Args(); err == nil {
		testing.ContextLog(ctx, "Running ", shutil.EscapeSlice(args))
	}
	report, err := t.Run(ctx)
	if err != nil {
		if report != nil {
			failedTests := report.FailedTestNames()
			for _, name := range failedTests {
				testing.ContextLog(ctx, "Failed test: ", name)
			}

			numFailed := len(failedTests)
			if numFailed == 1 {
				err = errors.Errorf("%s failed", failedTests[0])
			} else if numFailed > 1 {
				err = errors.Errorf("%s and %d more tests failed", failedTests[0], numFailed-1)
			}
		}
		return errors.Wrap(err, "failed to run cros_camera_test")
	}
	return nil
}

// getAvailableCameraHALsForTest returns a map from name to path for all camera
// HALs that are available for test.
func getAvailableCameraHALsForTest() (map[string]string, error) {
	cameraHALPaths, err := filepath.Glob(cameraHALGlobPattern)
	if err != nil {
		return nil, err
	}

	availableHALs := make(map[string]string)
	for _, path := range cameraHALPaths {
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if name == "usb" {
			if _, err := os.Stat(builtInUSBCameraConfigPath); os.IsNotExist(err) {
				// Ignore it in test because there is no built-in USB camera,
				// while it's installed for external USB cameras.
				continue
			} else if err != nil {
				return nil, err
			}
		}
		availableHALs[name] = path
	}
	return availableHALs, nil
}

// getCameraHALPathsForTest returns the paths for camera HALs specified.  If an
// empty slice is given, all available camera HALs are returned.
func getCameraHALPathsForTest(ctx context.Context, cameraHALs []string) ([]string, error) {
	availableHALs, err := getAvailableCameraHALsForTest()
	if err != nil {
		return nil, err
	}
	var paths []string
	if len(cameraHALs) == 0 {
		for _, path := range availableHALs {
			paths = append(paths, path)
		}
	} else {
		for _, hal := range cameraHALs {
			if path, ok := availableHALs[hal]; ok {
				paths = append(paths, path)
			} else {
				return nil, errors.Errorf("camera HAL %q is not available for test", hal)
			}
		}
	}
	return paths, nil
}

// parsePerfLog parses the performance log file generated by
// cros_camera_test.  Example output:
// Camera: front
// device_open: 5020 us
// preview_start: 353285 us
// still_image_capture: 308166 us
func parsePerfLog(ctx context.Context, path string, p *perf.Values) error {
	file, err := os.Open(path)
	if err != nil {
		return errors.Wrap(err, "failed to open log file")
	}
	defer file.Close()

	var camera string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.SplitN(line, ": ", 2)
		if len(tokens) != 2 {
			return errors.Errorf("expected 2 tokens in %q, but got %d", line, len(tokens))
		}
		if tokens[0] == "Camera" {
			camera = tokens[1]
			continue
		}
		name := fmt.Sprintf("camera_HAL3Perf.camera_%s_%s", camera, tokens[0])
		var value float64
		var unit string
		if _, err := fmt.Sscanf(tokens[1], "%f %s", &value, &unit); err != nil {
			return errors.Wrapf(err, "failed to parse value and unit from %q", tokens[1])
		}
		metric := perf.Metric{
			Name:      name,
			Unit:      unit,
			Direction: perf.SmallerIsBetter,
		}
		p.Set(metric, value)
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to scan perf log")
	}
	return nil
}

// RunTest runs cros_camera_test with proper environment setup and arguments
// according to the given config.
func RunTest(ctx context.Context, cfg TestConfig) (retErr error) {
	if len(cfg.CameraHALs) > 0 && len(cfg.CameraFacing) > 0 {
		return errors.New("cannot specify both CameraHALs and CameraFacing")
	}

	// Use a shorter context to save time for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if cfg.ConnectToCameraService {
		if upstart.JobExists(ctx, "cros-camera") {
			// Ensure that cros-camera service is running, because the service
			// might stopped due to the errors from some previous tests, and failed
			// to restart for some reasons.
			if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
				return errors.Wrap(err, "the cros-camera service is not running")
			}
		} else {
			return errors.New("failed to find the cros-camera service")
		}
	} else {
		testing.ContextLog(ctx, "Stopping cros-camera")
		if err := upstart.StopJob(shortCtx, "cros-camera"); err != nil {
			return errors.Wrap(err, "failed to stop cros-camera")
		}
		defer func() {
			testing.ContextLog(ctx, "Starting cros-camera")
			if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
				if retErr != nil {
					testing.ContextLog(ctx, "Failed to start cros-camera: ", err)
				} else {
					retErr = errors.Wrap(err, "failed to start cros-camera")
				}
			}
		}()
		if err := upstart.WaitForJobStatus(ctx, "cros-camera", upstartcommon.StopGoal,
			upstartcommon.WaitingState, upstart.RejectWrongGoal, ctxutil.MaxTimeout); err != nil {
			return errors.Wrap(err, "the cros-camera service did not stop before calling runCrosCameraTest")
		}
	}

	jsonCfg, err := json.Marshal(map[string]bool{
		"force_jpeg_hw_enc": cfg.ForceJPEGHWEnc,
		"force_jpeg_hw_dec": cfg.ForceJPEGHWDec,
	})
	if err != nil {
		return errors.Wrap(err, "failed to encode test config as json")
	}
	if err := ioutil.WriteFile(jsonConfigPath, jsonCfg, 0644); err != nil {
		return errors.Wrap(err, "failed to write json config file")
	}
	defer os.Remove(jsonConfigPath)

	cameraCfg := crosCameraTestConfig{
		gtestFilter:  cfg.GtestFilter,
		cameraFacing: cfg.CameraFacing,
	}

	if cfg.RequireRecordingParams {
		cameraCfg.recordingParams, err = getRecordingParams(shortCtx)
		if err != nil {
			return errors.Wrap(err, "failed to get recording params")
		}
	}

	if cfg.PortraitModeTestData != "" {
		cameraCfg.portraitModeTestData = cfg.PortraitModeTestData
	}

	p := perf.NewValues()
	updatePerfIfNeeded := func() error {
		if cameraCfg.perfLog != "" {
			if err := parsePerfLog(ctx, cameraCfg.perfLog, p); err != nil {
				return errors.Wrap(err, "failed to parse perf log")
			}
		}
		return nil
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("missing out dir")
	}

	if len(cfg.CameraFacing) > 0 || cfg.ConnectToCameraService {
		if cfg.GeneratePerfLog {
			cameraCfg.perfLog = filepath.Join(outDir, "perf.log")
		}
		if err := runCrosCameraTest(shortCtx, cameraCfg); err != nil {
			return err
		}
		if err := updatePerfIfNeeded(); err != nil {
			return err
		}
	} else {
		paths, err := getCameraHALPathsForTest(shortCtx, cfg.CameraHALs)
		if err != nil {
			return errors.Wrap(err, "failed to get paths of camera HALs")
		}

		for i, path := range paths {
			cameraCfg.cameraHALPath = path
			filepath.Base(path)
			if cfg.GeneratePerfLog {
				cameraCfg.perfLog = filepath.Join(outDir, fmt.Sprintf("perf_%d.log", i))
			}
			if err := runCrosCameraTest(shortCtx, cameraCfg); err != nil {
				return err
			}
			if err := updatePerfIfNeeded(); err != nil {
				return err
			}
		}
	}
	if cfg.GeneratePerfLog {
		if err := p.Save(outDir); err != nil {
			return errors.Wrap(err, "failed to save perf data")
		}
	}
	return nil
}
