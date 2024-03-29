// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hal3

// TestConfig is the config for HAL3 tests.
type TestConfig struct {
	// CameraHALs is a list of camera HALs to test, such as "usb".  If
	// unspecified, all available camera HALs would be tested.
	CameraHALs []string
	// CameraFacing is the facing of the camera to test, such as "front" or
	// "back".  This field and CameraHALs are mutually exclusive.
	CameraFacing string
	// GtestFilter would be passed to cros_camera_test as the value of
	// --gtest_filter command line switch.
	GtestFilter string
	// GeneratePerfLog describes whether the performance log file should be
	// generated by cros_camera_test.
	GeneratePerfLog bool
	// RequireRecordingParams describes whether the recording parameters should
	// be generated for cros_camera_test.
	RequireRecordingParams bool
	// ForceJPEGHWEnc is the flag to enforce hardware encode for JPEG, so it
	// won't fall back to SW encode when the HW encode failed.
	ForceJPEGHWEnc bool
	// ForceJPEGHWDec is the flag to enforce hardware decode for JPEG, so it
	// won't fall back to SW decode when the HW decode failed.
	ForceJPEGHWDec bool
	// ConnectToCameraService is the flag to connect to the cros-camera service,
	// instead of loading camera HALs.
	ConnectToCameraService bool
	// PortraitModeTestData is the portrait mode test data to be downloaded.
	PortraitModeTestData string
	// Number of faces should be detected for face detection test.
	ExpectedNumFaces string
}

// DeviceTestConfig returns test config for running HAL3Device test.
func DeviceTestConfig() TestConfig {
	return TestConfig{
		GtestFilter: "Camera3DeviceTest/*",
	}
}

// FrameTestConfig returns test config for running HAL3Frame test.
func FrameTestConfig() TestConfig {
	return TestConfig{
		GtestFilter: "Camera3FrameTest/*",
	}
}

// JDATestConfig returns test config for running HAL3JDA test.
func JDATestConfig() TestConfig {
	return TestConfig{
		CameraHALs:     []string{"usb"},
		GtestFilter:    "*/Camera3SingleFrameTest.GetFrame/0",
		ForceJPEGHWDec: true,
	}
}

// JEAUSBTestConfig returns test config for running HAL3JEA test on USB HAL.
func JEAUSBTestConfig() TestConfig {
	return TestConfig{
		CameraHALs:     []string{"usb"},
		GtestFilter:    "*/Camera3SimpleStillCaptureTest.TakePictureTest/0",
		ForceJPEGHWEnc: true,
	}
}

// JEATestConfig returns test config for running HAL3JEA test.
func JEATestConfig() TestConfig {
	return TestConfig{
		GtestFilter:    "*/Camera3SimpleStillCaptureTest.TakePictureTest/0",
		ForceJPEGHWEnc: true,
	}
}

// ModuleTestConfig returns test config for running HAL3Module test.
func ModuleTestConfig() TestConfig {
	return TestConfig{
		GtestFilter:            "Camera3ModuleFixture.*",
		RequireRecordingParams: true,
	}
}

// PerfTestConfig returns test config for running HAL3Perf test.
func PerfTestConfig() TestConfig {
	return TestConfig{
		GtestFilter:     "Camera3StillCaptureTest/Camera3SimpleStillCaptureTest.PerformanceTest/*",
		GeneratePerfLog: true,
	}
}

// PreviewTestConfig returns test config for running HAL3Preview test.
func PreviewTestConfig() TestConfig {
	return TestConfig{
		GtestFilter: "Camera3PreviewTest/*",
	}
}

// RecordingTestConfig returns test config for running HAL3Recording test.
func RecordingTestConfig() TestConfig {
	return TestConfig{
		GtestFilter:            "Camera3RecordingFixture/*",
		RequireRecordingParams: true,
	}
}

// StillCaptureTestConfig returns test config for running HAL3StillCapture test.
func StillCaptureTestConfig() TestConfig {
	return TestConfig{
		GtestFilter: "Camera3StillCaptureTest/*",
	}
}

// StreamTestConfig returns test config for running HAL3Stream test.
func StreamTestConfig() TestConfig {
	return TestConfig{
		GtestFilter: "Camera3StreamTest/*",
	}
}

// PortraitModeTestConfig returns test config for running HAL3PortraitMode test.
func PortraitModeTestConfig(generatePerfLog bool, portraitModeTestFile string) TestConfig {
	return TestConfig{
		GtestFilter:            "Camera3FrameTest/Camera3PortraitModeTest.*",
		ConnectToCameraService: true,
		GeneratePerfLog:        generatePerfLog,
		PortraitModeTestData:   portraitModeTestFile,
	}
}

// StillCaptureZSLTestConfig returns test config for running HAL3StillCaptureZSL test.
func StillCaptureZSLTestConfig() TestConfig {
	return TestConfig{
		GtestFilter:            "Camera3StillCaptureTest/Camera3SimpleStillCaptureTest.TakePictureZslTest/*",
		ConnectToCameraService: true,
	}
}

// AUETestConfig returns test config for running HAL3AUE test.
func AUETestConfig() TestConfig {
	return TestConfig{
		// For now AUE test is only covering Camera3Preview functionality.
		// It can be expanded to cover other functionalities.
		// To cover multiple tests, separate the filters with semicolon.
		GtestFilter: "Camera3PreviewTest/*",
	}
}
