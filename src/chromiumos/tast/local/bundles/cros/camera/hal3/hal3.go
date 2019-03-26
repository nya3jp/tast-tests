// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hal3

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	builtInUSBCameraConfigPath = "/etc/camera/camera_characteristics.conf"
	cameraHALGlobPattern       = "/usr/lib*/camera_hal/*.so"
	jsonConfigPath             = "/var/cache/camera/test_config.json"
	mediaProfilePath           = "/vendor/etc/media_profiles.xml"
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

// getRecordingParams gets the recording parameters from the media profile in
// ARC, which would be used as an argument of cros_camera_test.
func getRecordingParams(ctx context.Context) (string, error) {
	arcCmd := shutil.EscapeSlice([]string{"cat", mediaProfilePath})
	cmd := testexec.CommandContext(ctx, "android-sh", "-c", arcCmd)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", err
	}
	var settings mediaSettings
	if err := xml.Unmarshal(out, &settings); err != nil {
		return "", err
	}
	seen := make(map[string]struct{})
	var params []string
	for _, cprof := range settings.CamcorderProfiles {
		for _, eprof := range cprof.EncoderProfile {
			video := eprof.Video
			param := fmt.Sprintf("%d:%d:%d:%d", cprof.CameraID,
				video.Width, video.Height, video.FrameRate)
			if _, ok := seen[param]; !ok {
				seen[param] = struct{}{}
				params = append(params, param)
			}
		}
	}
	return strings.Join(params, ","), nil
}

// crosCameraTestConfig is the config for running cros_camera_test.
type crosCameraTestConfig struct {
	cameraHALPath   string
	cameraFacing    string
	gtestFilter     string
	recordingParams string
}

// toArgs converts crosCameraTestConfig to a list of argument strings.
func (t *crosCameraTestConfig) toArgs() []string {
	var args []string
	if t.cameraHALPath != "" {
		args = append(args, "--camera_hal_path="+t.cameraHALPath)
	}
	if t.cameraFacing != "" {
		args = append(args, "--camera_facing="+t.cameraFacing)
	}
	if t.gtestFilter != "" {
		args = append(args, "--gtest_filter="+t.gtestFilter)
	}
	if t.recordingParams != "" {
		args = append(args, "--recording_params="+t.recordingParams)
	}
	return args
}

// gtestResult is used to unmarshal GoogleTest XML output files.
type gtestResult struct {
	XMLName xml.Name `xml:"testsuites"`
	Suites  []struct {
		Cases []struct {
			Name      string        `xml:"name,attr"`
			ClassName string        `xml:"classname,attr"`
			Failures  []interface{} `xml:"failure"`
		} `xml:"testcase"`
	} `xml:"testsuite"`
}

// GetFailedTestNames returns failed test names from the gtest xml output file.
// TODO(shik): Consolidate gtest related helpers in one place.  There is another
// similar one that uses json output in chromiumos/tast/local/chrome/bintest.
func GetFailedTestNames(r io.Reader) ([]string, error) {
	out, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var res gtestResult
	if err := xml.Unmarshal(out, &res); err != nil {
		return nil, err
	}

	var names []string
	for _, suite := range res.Suites {
		for _, cas := range suite.Cases {
			if len(cas.Failures) == 0 {
				continue
			}
			name := fmt.Sprintf("%s.%s", cas.ClassName, cas.Name)
			names = append(names, name)
		}
	}
	return names, nil
}

// runCrosCameraTest runs cros_camera_test with the arguments generated from the
// config.  The cros-camera service must be stopped before calling this function.
func runCrosCameraTest(ctx context.Context, s *testing.State, cfg crosCameraTestConfig) {
	if err := upstart.WaitForJobStatus(ctx, "cros-camera", upstart.StopGoal,
		upstart.WaitingState, upstart.RejectWrongGoal, 0); err != nil {
		s.Fatal("The cros-camera service must be stopped before calling runCrosCameraTest: ", err)
	}

	gtestFile, err := ioutil.TempFile(s.OutDir(), "gtest.*.xml")
	if err != nil {
		s.Fatal("Failed to open gtest output file: ", err)
	}
	defer func() {
		if err := gtestFile.Close(); err != nil {
			s.Error("Failed to close gtest output file: ", err)
		}
	}()

	logPath := filepath.Join(s.OutDir(), "cros_camera_test.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Fatal("Failed to open log file: ", err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			s.Error("Failed to close log file: ", err)
		}
	}()

	cmd := testexec.CommandContext(ctx, "cros_camera_test", cfg.toArgs()...)
	cmd.Env = []string{"GTEST_OUTPUT=xml:" + gtestFile.Name()}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	msg := "Running " + shutil.EscapeSlice(cmd.Args)
	s.Log(msg)

	// Make it easier to read by writing the command to the log file as well,
	// because we might run cros_camera_test multiple times in a Tast test.
	logFile.WriteString(msg + "\n")
	if err := logFile.Sync(); err != nil {
		s.Error("Failed to flush log file: ", err)
	}

	if err := cmd.Run(); err != nil {
		if names, err := GetFailedTestNames(gtestFile); err != nil {
			s.Error("Failed to extract failed test names: ", err)
		} else {
			for _, name := range names {
				s.Error(name, " failed")
			}
		}
		s.Fatal("Failed to run cros_camera_test: ", err)
	}

}

// TestConfig is the config for HAL3 tests.
type TestConfig struct {
	// CameraHALs is an list of camera HALs to test, such as "usb".  If
	// unspecified, all available camera HALs would be tested.
	CameraHALs []string
	// CameraFacing is the facing of the camera to test, such as "front" or
	// "back".  This field and CameraHALs are mutually exclusive.
	CameraFacing string
	// GtestFilter would be passed to cros_camera_test as the value of
	// --gtest_filter command line switch.
	GtestFilter string
	// RequireRecordingParams describes whether the recording parameters should
	// be generated for cros_camera_test.
	RequireRecordingParams bool
	// ForceJPEGHWEnc is the flag to enforce hardware encode for JPEG, so it
	// won't fall back to SW encode when the HW encode failed.
	ForceJPEGHWEnc bool
	// ForceJPEGHWDec is the flag to enforce hardware decode for JPEG, so it
	// won't fall back to SW decode when the HW decode failed.
	ForceJPEGHWDec bool
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

// RunTest runs cros_camera_test with proper environment setup and arguments
// according to the given config.
func RunTest(ctx context.Context, s *testing.State, cfg TestConfig) {
	if len(cfg.CameraHALs) > 0 && len(cfg.CameraFacing) > 0 {
		s.Fatal("Cannot specify both CameraHALs and CameraFacing")
	}

	// Use a shorter context to save time for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Log("Stopping cros-camera")
	if err := upstart.StopJob(shortCtx, "cros-camera"); err != nil {
		s.Fatal("Failed to stop cros-camera: ", err)
	}
	defer func() {
		s.Log("Starting cros-camera")
		if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
			s.Error("Failed to start cros-camera: ", err)
		}
	}()

	jsonCfg, err := json.Marshal(map[string]bool{
		"force_jpeg_hw_enc": cfg.ForceJPEGHWEnc,
		"force_jpeg_hw_dec": cfg.ForceJPEGHWDec,
	})
	if err != nil {
		s.Fatal("Failed to encode test config as json: ", err)
	}
	if err := ioutil.WriteFile(jsonConfigPath, jsonCfg, 0644); err != nil {
		s.Fatal("Failed to write json config file: ", err)
	}
	defer os.Remove(jsonConfigPath)

	cameraCfg := crosCameraTestConfig{
		gtestFilter:  cfg.GtestFilter,
		cameraFacing: cfg.CameraFacing,
	}

	if cfg.RequireRecordingParams {
		cameraCfg.recordingParams, err = getRecordingParams(shortCtx)
		if err != nil {
			s.Fatal("Failed to get recording params: ", err)
		}
	}

	if len(cfg.CameraFacing) > 0 {
		runCrosCameraTest(shortCtx, s, cameraCfg)
	} else {
		paths, err := getCameraHALPathsForTest(shortCtx, cfg.CameraHALs)
		if err != nil {
			s.Fatal("Failed to get paths of camera HALs: ", err)
		}

		for _, path := range paths {
			cameraCfg.cameraHALPath = path
			runCrosCameraTest(shortCtx, s, cameraCfg)
		}
	}
}
