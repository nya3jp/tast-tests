// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package logging provides a log helper in video tests.
package logging

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/testing"
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
	// Files is a list of file paths to write a value to for verbose logging in a video driver.
	files []string
	// EnableValue is data written to Files to enable verbose logging.
	enableValue []byte
	// DisableValue is data written to Files to disable verbose logging.
	disableValue []byte
	// Desc is a description about a driver in log.
	desc string
}

// getLogSpecs returns a list of logSpec which represents files and values to be written on DUT.
func getLogSpecs(ctx context.Context) ([]logSpec, error) {
	var specs []logSpec

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
			testing.ContextLog(ctx, "Failed to match: ", l.glob)
			return nil, err
		}
		if len(files) > 0 {
			specs = append(specs, logSpec{files, []byte(l.enable), []byte(l.disable), l.desc})
		}
	}
	return specs, nil
}

// VideoLogger enables/disables verbose logging in a video driver properly during the test.
type VideoLogger struct {
	// specs is a list of logSpec for enabled/disabled logging.
	specs []logSpec
}

// NewVideoLogger enables verbose logging in a video driver and returns a VideoLogger.
// VideoLogger.Close disables the verbose logging.
func NewVideoLogger(ctx context.Context) (*VideoLogger, error) {
	var vl VideoLogger
	var err error
	vl.specs, err = getLogSpecs(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range vl.specs {
		for _, f := range l.files {
			if err := ioutil.WriteFile(f, l.enableValue, 0644); err != nil {
				testing.ContextLog(ctx, "Setting log level failed: ", err)
				return nil, err
			}
		}
	}
	return &vl, nil
}

// Close disables verbose logging in a video driver which are enabled in NewVideoLogger.
func (vl *VideoLogger) Close(ctx context.Context) error {
	for _, l := range vl.specs {
		for _, f := range l.files {
			// Be sure |f| exists because it is acquired by Glob().
			if err := ioutil.WriteFile(f, l.disableValue, 0644); err != nil {
				testing.ContextLog(ctx, "Setting log level failed: ", err)
				return err
			}
		}
	}
	return nil
}
