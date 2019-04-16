// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package logging controls logging in video drivers.
package logging

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
)

// ChromeVmoduleFlag returns a command line option for Chrome (binary test) to
// enable logging in media/gpu code.
func ChromeVmoduleFlag() string {
	loggingPatterns := []string{
		"*/media/gpu/*video_decode_accelerator.cc=2",
		"*/media/gpu/*video_encode_accelerator.cc=2",
		"*/media/gpu/*jpeg_decode_accelerator.cc=2",
		"*/media/gpu/*jpeg_encode_accelerator.cc=2",
		"*/media/gpu/*image_processor.cc=2",
		"*/media/gpu/*v4l2_device.cc=2",
	}
	return "--vmodule=" + strings.Join(loggingPatterns, ",")
}

// logSpec denotes the information to enable verbose logging in a video driver.
type logSpec struct {
	files        []string // files to write a value to to control verbose logging in a video driver.
	enableValue  []byte   // data written to files to enable verbose logging.
	disableValue []byte   // data written to files to disable verbose logging.
	desc         string   // human-readable description of driver.
}

// getLogSpecs returns a list of logSpec which represents files and values to be written on DUT.
func getLogSpecs() ([]logSpec, error) {
	var specs []logSpec

	for _, l := range []struct {
		glob, enable, disable, desc string
	}{
		{"/sys/module/s5p_mfc/parameters/debug", "1", "0", "s5p_mfc"},
		{"/sys/module/go2001/parameters/go2001_debug_level", "1", "0", "go2001"},

		// The debug level is a bitfield, with 3 enabling log levels 0 and 1
		{"/sys/module/rockchip_vpu/parameters/debug", "3", "0", "rk3399"},
		{"/sys/module/rk3288_vpu/parameters/debug", "3", "0", "rk3288"},
	} {
		files, err := filepath.Glob(l.glob)
		if err != nil {
			return nil, errors.Wrapf(err, "bad glob %q", l.glob)
		}
		if len(files) > 0 {
			specs = append(specs, logSpec{files, []byte(l.enable), []byte(l.disable), l.desc})
		}
	}
	return specs, nil
}

// VideoLogger enables/disables verbose logging in a video driver properly during the test.
type VideoLogger struct {
	specs []logSpec
}

// NewVideoLogger enables verbose logging in a video driver and returns a VideoLogger.
// VideoLogger.Close disables the verbose logging.
func NewVideoLogger() (*VideoLogger, error) {
	specs, err := getLogSpecs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log specs")
	}
	vl := VideoLogger{specs}

	for _, l := range vl.specs {
		for _, f := range l.files {
			if err := ioutil.WriteFile(f, l.enableValue, 0644); err != nil {
				vl.Close()
				return nil, errors.Wrap(err, "failed to set log level")
			}
		}
	}
	return &vl, nil
}

// Close disables verbose logging.
func (vl *VideoLogger) Close() error {
	var lastErr error
	for _, l := range vl.specs {
		for _, f := range l.files {
			if err := ioutil.WriteFile(f, l.disableValue, 0644); err != nil {
				lastErr = errors.Wrap(err, "failed to set log level")
			}
		}
	}
	return lastErr
}
