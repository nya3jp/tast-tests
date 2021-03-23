// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package c2e2etest contains constants and utilities for the prebuilt android test APK.
package c2e2etest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

const (
	// Pkg is the package name of the test APK
	Pkg = "org.chromium.c2.test"
	// ActivityName is the name of the test activity
	ActivityName = ".E2eTestActivity"

	// X86ApkName is the name of the c2_e2e_test apk for x86/x86_64 devices
	X86ApkName = "c2_e2e_test_x86.apk"
	// ArmApkName is the name of the c2_e2e_test apk for arm devices
	ArmApkName = "c2_e2e_test_arm.apk"
)

// ApkNameForArch gets the name of the APK file to install on the DUT.
func ApkNameForArch(ctx context.Context, a *arc.ARC) (string, error) {
	out, err := a.Command(ctx, "getprop", "ro.product.cpu.abi").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get abi: %v", err)
	}

	if strings.HasPrefix(string(out), "x86") {
		return X86ApkName, nil
	}
	return ArmApkName, nil
}

// RequiredPermissions returns the array of permissions necessary for the test APK.
func RequiredPermissions() []string {
	return []string{
		"android.permission.READ_EXTERNAL_STORAGE",
		"android.permission.WRITE_EXTERNAL_STORAGE"}
}

// GrantApkPermissions grants the permissions necessary for the test APK.
func GrantApkPermissions(ctx context.Context, a *arc.ARC) error {
	for _, perm := range RequiredPermissions() {
		if err := a.Command(ctx, "pm", "grant", Pkg, perm).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to grant permission: %v", err)
		}
	}
	return nil
}

// PullLogs pulls the specified gtest log files
func PullLogs(ctx context.Context, a *arc.ARC, arcFilePath, localFilePath, localFilePrefix, textLogName, xmlLogName string) (outLogFile, outXMLFile string, err error) {
	outLogFile = fmt.Sprintf("%s/%s%s", localFilePath, localFilePrefix, textLogName)
	outXMLFile = fmt.Sprintf("%s/%s%s", localFilePath, localFilePrefix, xmlLogName)

	if err := a.PullFile(ctx, filepath.Join(arcFilePath, textLogName), outLogFile); err != nil {
		return "", "", errors.Wrapf(err, "failed fo pull %s", textLogName)
	}

	if err := a.PullFile(ctx, filepath.Join(arcFilePath, xmlLogName), outXMLFile); err != nil {
		return "", "", errors.Wrapf(err, "failed fo pull %s", xmlLogName)
	}
	return outLogFile, outXMLFile, nil
}

// ValidateXMLLogs validates the given xml gtest log file
func ValidateXMLLogs(xmlLogFile string) error {
	r, err := gtest.ParseReport(xmlLogFile)
	if err != nil {
		return err
	}

	// Walk through the whole report and collect failed test cases and their messages.
	var failures []string
	space := regexp.MustCompile(`\s+`)
	for _, s := range r.Suites {
		for _, c := range s.Cases {
			if len(c.Failures) > 0 {
				// Report only the first error message as one line for each test case.
				msg := space.ReplaceAllString(c.Failures[0].Message, " ")
				failures = append(failures, fmt.Sprintf("\"%s.%s: %s\"", s.Name, c.Name, msg))
			}
		}
	}
	if failures != nil {
		return errors.Errorf("c2_e2e_test failed: %s", failures)
	}

	return nil
}

// VideoMetadata stores parsed metadata from test video JSON files, which are external files located in
// gs://chromiumos-test-assets-public/tast/cros/video/, e.g. test-25fps.h264.json.
type VideoMetadata struct {
	Profile      string   `json:"profile"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	FrameRate    int      `json:"frame_rate"`
	NumFrames    int      `json:"num_frames"`
	NumFragments int      `json:"num_fragments"`
	MD5Checksums []string `json:"md5_checksums"`
}

// LoadMetadata loads a video's metadata from a file.
func LoadMetadata(filePath string) (*VideoMetadata, error) {
	var md VideoMetadata
	jf, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed load metadata")
	}
	defer jf.Close()

	if err := json.NewDecoder(jf).Decode(&md); err != nil {
		return nil, errors.Wrapf(err, "parsing %s to load metadata", filePath)
	}

	return &md, nil
}

// videoCodecEnumValues maps profile string to its enum value.
// These values must match integers in VideoCodecProfile in https://cs.chromium.org/chromium/src/media/base/video_codecs.h
var videoCodecEnumValues = map[string]int{
	"H264PROFILE_MAIN":    1,
	"VP8PROFILE_ANY":      11,
	"VP9PROFILE_PROFILE0": 12,
}

// StreamDataArg returns a string that can be used for an argument to the c2_e2e_test APK.
// dataPath is the absolute path of the video file.
func (d *VideoMetadata) StreamDataArg(dataPath string) (string, error) {
	pEnum, found := videoCodecEnumValues[d.Profile]
	if !found {
		return "", errors.Errorf("cannot find enum value for profile %v", d.Profile)
	}

	// Set MinFPSNoRender and MinFPSWithRender to 0 for disabling FPS check because we would like
	// TestFPS to be always passed and store FPS value into perf metric.
	sdArg := fmt.Sprintf("--test_video_data=%s:%d:%d:%d:%d:0:0:%d:%d",
		dataPath, d.Width, d.Height, d.NumFrames, d.NumFragments, pEnum, d.FrameRate)
	return sdArg, nil
}

// WaitForCodecReady waits for an already running E2eTestActivity to finish setup of its codec.
func WaitForCodecReady(ctx context.Context, a *arc.ARC) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		res, err := a.BroadcastIntent(ctx, "org.chromium.c2.test.CHECK_CODEC_CONFIGURED", Pkg)
		if err != nil {
			return testing.PollBreak(err)
		}
		if res.Result != 1 {
			return errors.Errorf("Codec not yet configured: %d", res.Result)
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})
}
