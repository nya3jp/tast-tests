package hal3

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	builtInUsbCameraConfigPath = "/etc/camera/camera_characteristics.conf"
	cameraHalGlobPattern       = "/usr/lib*/camera_hal/*.so"
	jsonConfigPath             = "/var/cache/camera/test_config.json"
	mediaProfilePath           = "/vendor/etc/media_profiles.xml"
)

// mediaSettings is a helper struct for accessing media profile.
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
// ARC++, which would be used as an argument of cros_camera_test.
func getRecordingParams(ctx context.Context) (string, error) {
	arcCmd := shutil.EscapeSlice([]string{"cat", mediaProfilePath})
	cmd := testexec.CommandContext(ctx, "android-sh", "-c", arcCmd)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", err
	}
	mediaSettings := mediaSettings{}
	if err := xml.Unmarshal(out, &mediaSettings); err != nil {
		return "", err
	}
	seen := make(map[string]bool)
	var params []string
	for _, camcorderProfile := range mediaSettings.CamcorderProfiles {
		for _, encoderProfile := range camcorderProfile.EncoderProfile {
			video := encoderProfile.Video
			param := fmt.Sprintf("%d:%d:%d:%d", camcorderProfile.CameraID,
				video.Width, video.Height, video.FrameRate)
			if !seen[param] {
				seen[param] = true
				params = append(params, param)
			}
		}
	}
	return strings.Join(params, ","), nil
}

// crosCameraTestConfig is the config for running cros_camera_test.
type crosCameraTestConfig struct {
	CameraHalPath   string
	CameraFacing    string
	GtestFilter     string
	RecordingParams string
}

// toArgsList converts crosCameraTestConfig to a list of argument strings.
func (t *crosCameraTestConfig) toArgs() []string {
	args := []string{}
	addArgf := func(format string, a ...interface{}) {
		args = append(args, fmt.Sprintf(format, a...))
	}
	if t.CameraHalPath != "" {
		addArgf("--camera_hal_path=%s", t.CameraHalPath)
	}
	if t.GtestFilter != "" {
		addArgf("--gtest_filter=%s", t.GtestFilter)
	}
	if t.RecordingParams != "" {
		addArgf("--recording_params=%s", t.RecordingParams)
	}
	return args
}

// runCrosCameraTest runs cros_camera_test with the arguments generated from the
// config.  The cros-camera service must be stopped before calling this function.
func runCrosCameraTest(ctx context.Context, s *testing.State, cfg crosCameraTestConfig) {
	if err := upstart.WaitForJobStatus(ctx, "cros-camera", upstart.StopGoal,
		upstart.WaitingState, upstart.RejectWrongGoal, 0); err != nil {
		s.Fatal("The cros-camera service must be stopped")
	}

	cmd := testexec.CommandContext(ctx, "cros_camera_test", cfg.toArgs()...)
	s.Logf("Running %s", shutil.EscapeSlice(cmd.Args))
	err := cmd.Run()

	// TODO(shik): Dump the log on failure only.  Currently some tests are quite
	// flaky and we relies on the logs of successful runs when diagnosing.
	cmd.DumpLog(ctx)

	if err != nil {
		s.Fatal("Failed to run cros_camera_test: ", err)
	}
}

// TestConfig is the config for HAL3 tests.
type TestConfig struct {
	CameraHals             []string `json:"-"`
	CameraFacing           string   `json:"-"`
	GtestFilter            string   `json:"-"`
	RequireRecordingParams bool     `json:"-"`
	ForceJpegHwEnc         bool     `json:"force_jpeg_hw_enc,omitempty"`
	ForceJpegHwDec         bool     `json:"force_jpeg_hw_dec,omitempty"`
}

// getAvailableCameraHalsForTest returns a map from name to path for all camera
// HALs that are available for test.
func getAvailableCameraHalsForTest() (map[string]string, error) {
	cameraHalPaths, err := filepath.Glob(cameraHalGlobPattern)
	if err != nil {
		return nil, err
	}

	availableHals := make(map[string]string)
	for _, path := range cameraHalPaths {
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if name == "usb" {
			_, err := os.Stat(builtInUsbCameraConfigPath)
			if os.IsNotExist(err) {
				// Ignore it in test because there is no built-in USB camera,
				// while it's installed for external USB cameras.
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		availableHals[name] = path
	}
	return availableHals, nil
}

// getCameraHalPathsForTest returns the paths for camera HALs specified.  If an
// empty slice is given, all available camera HALs are returned.
func getCameraHalPathsForTest(ctx context.Context, cameraHals []string) ([]string, error) {
	availableHals, err := getAvailableCameraHalsForTest()
	if err != nil {
		return nil, err
	}
	var cameraHalPathsForTest []string
	if len(cameraHals) == 0 {
		for _, path := range availableHals {
			cameraHalPathsForTest = append(cameraHalPathsForTest, path)
		}
	} else {
		for _, cameraHal := range cameraHals {
			path, ok := availableHals[cameraHal]
			if ok {
				cameraHalPathsForTest = append(cameraHalPathsForTest, path)
			} else {
				testing.ContextLogf(ctx, "Camera HAL %q is not available for test", cameraHal)
			}
		}
	}
	return cameraHalPathsForTest, nil
}

// RunTest runs cros_camera_test with proper environment setup and arguments
// accroding to the given config.
func RunTest(ctx context.Context, s *testing.State, cfg TestConfig) {
	if len(cfg.CameraHals) > 0 && len(cfg.CameraFacing) > 0 {
		s.Fatal("Cannot specify both CameraHals and CameraFacing")
	}

	// Use a shorter context to save time for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Log("Stopping cros-camera")
	if err := upstart.StopJob(shortCtx, "cros-camera"); err != nil {
		s.Log("Failed to stop cros-camera: ", err)
	}
	defer func() {
		s.Log("Starting cros-camera")
		if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
			s.Log("Failed to start cros-camera: ", err)
		}
	}()

	jsonConfig, err := json.Marshal(cfg)
	if err != nil {
		s.Fatal("Failed to encode test config as json: ", err)
	}
	if err := ioutil.WriteFile(jsonConfigPath, jsonConfig, 0644); err != nil {
		s.Fatal("Failed to write json config file: ", err)
	}
	defer os.Remove(jsonConfigPath)

	newCfg := crosCameraTestConfig{
		GtestFilter:  cfg.GtestFilter,
		CameraFacing: cfg.CameraFacing,
	}

	if cfg.RequireRecordingParams {
		newCfg.RecordingParams, err = getRecordingParams(shortCtx)
		if err != nil {
			s.Fatal("Failed to get recording params: ", err)
		}
	}

	if len(cfg.CameraFacing) > 0 {
		runCrosCameraTest(ctx, s, newCfg)
	} else {
		cameraHalPathsForTest, err := getCameraHalPathsForTest(shortCtx, cfg.CameraHals)
		if err != nil {
			s.Fatal("Failed to get paths of camera HALs: ", err)
		}

		for _, cameraHalPath := range cameraHalPathsForTest {
			newCfg.CameraHalPath = cameraHalPath
			runCrosCameraTest(ctx, s, newCfg)
		}
	}
}
