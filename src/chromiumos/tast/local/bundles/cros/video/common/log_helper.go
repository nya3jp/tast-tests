// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/testing"
)

// ChromeVmoduleFlag return a command line option for Chrome (binary test) to
// enable verbose log in media/gpu code.
func ChromeVmoduleFlag() string {
	loggingPatterns := []string{
		"*/media/gpu/*video_decode_accelerator.cc=2",
		"*/media/gpu/*video_encode_accelerator.cc=2",
		"*/media/gpu/*jpeg_decode_accelerator.cc=2",
		"*/media/gpu/*jpeg_encode_accelerator.cc=2",
		"*/media/gpu/*image_processor.cc=2",
		"*/media/gpu/*v4l2_device.cc=2",
	}
	return "--vmodule=" + strings.Join(loggingPatterns[:], ",")
}

// LogInfo denotes the information to enable verbose log in a video driver only
// during a video test.
type LogInfo struct {
	// List of filepaths to write a value for verbose log in a video driver.
	Files []string
	// A set value in the beginning of a video test to enable verbose log.
	EnableValue []byte
	// A set value in the beginning of a video test to disable verbose log.
	DisableValue []byte
	// Description about a driver in log.
	Desc string
}

// getLogInfos returns LogInfo[] which represents files and values to be written on DUT.
func getLogInfos(s *testing.State) []LogInfo {
	info := []LogInfo{}

	//videobuf2 log
	for _, l := range []struct {
		glob, enable, disable, desc string
	}{
		{"/sys/module/videobuf2_*/parameters/debug", "1", "0", "videobuf2"},
		{"/sys/module/s5p_mfc/parameters/debug", "1", "0", "s5p_mfc"},
		{"/sys/module/go2001/parameters/go2001_debug_level", "1", "0", "go2001"},

		// The debug level is a bitfield, with 3 enabling log levels 0 and 1
		{"/sys/module/rockchip_vpu/parameters/debug", "3", "0", "rk3399"},
		{"/sys/module/rk3288_vpu/parameters/debug", "3", "0", "rk3288"},
	} {
		files, err := filepath.Glob(l.glob)
		if err != nil {
			s.Error("Failed in Glob: ", l.glob)
			continue
		}
		info = append(info, LogInfo{files, []byte(l.enable), []byte(l.disable), l.desc})
	}
	return info
}

// EnableVideoLogs enables verbose log in a video driver.
// Returns LogInfo[] should directly pass to DisableVideoLogs().
// The typical use pattern is "defer common.DisableVideoLogs(common.EnableVideoLogs(s))"
func EnableVideoLogs(s *testing.State) (*testing.State, []LogInfo) {
	info := getLogInfos(s)
	for _, l := range info {
		for _, f := range l.Files {
			s.Logf("setting %v log level to %v", l.Desc, l.EnableValue)
			// Be sure |f| exists because it is acquired by Glob().
			if err := ioutil.WriteFile(f, l.EnableValue, 0644); err != nil {
				s.Error("Setting log level failed: ", err)
			}
		}
	}
	return s, info
}

// DisableVideoLogs disables verbose log in a video driver.
func DisableVideoLogs(s *testing.State, info []LogInfo) {
	for _, l := range info {
		for _, f := range l.Files {
			s.Logf("resetting %v log level to %v", l.Desc, l.DisableValue)
			// Be sure |f| exists because it is acquired by Glob().
			if err := ioutil.WriteFile(f, l.DisableValue, 0644); err != nil {
				s.Error("Setting log level failed: ", err)
			}
		}
	}
}
