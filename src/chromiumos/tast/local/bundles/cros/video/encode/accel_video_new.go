// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package encode provides common code to run Chrome binary tests for video encoding.
package encode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// TestOptionsNew is the options for runNewAccelVideoTest.
type TestOptionsNew struct {
	WebMName string
	Profile  videotype.CodecProfile
}

// JSONFileName returns the json file name used in video.EncodeAccelNew with |webmMFileName|.
// For example, if |webMFileName| is bear-320x192.vp9.webm, then bear-320x192.yuv.json is returned.
func JSONFileName(webMFileName string) string {
	const webMSuffix = ".vp9.webm"
	if !strings.HasSuffix(webMFileName, webMSuffix) {
		return "error.json"
	}
	yuvName := strings.TrimSuffix(webMFileName, webMSuffix) + ".yuv"
	return yuvName + ".json"
}

func codecProfileToEncodeCodecOption(profile videotype.CodecProfile) (string, error) {
	switch profile {
	case videotype.H264Prof:
		return "h264baseline", nil
	case videotype.VP8Prof:
		return "vp8", nil
	case videotype.VP9Prof:
		return "vp9", nil
	default:
		return "", errors.Errorf("unknown codec profile: %v", profile)
	}
}

// RunNewAccelVideoTest runs all tests in video_encode_accelerator_tests.
func RunNewAccelVideoTest(ctxForDefer context.Context, s *testing.State, opts TestOptionsNew) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	ctx, cancel := ctxutil.Shorten(ctxForDefer, 10*time.Second)
	defer cancel()

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	yuvPath, err := encoding.PrepareYUV(ctx, s.DataPath(opts.WebMName), videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(yuvPath)

	jsonFileName := JsonFileName(opts.WebMName)
	yuvJSONPath := yuvPath + ".json"
	if err := fsutil.CopyFile(s.DataPath(jsonFileName), yuvJSONPath); err != nil {
		s.Fatal("Failed to copy json file: ", s.DataPath(jsonFileName), yuvJSONPath)
	}
	defer os.Remove(yuvJSONPath)
	codec, err := codecProfileToEncodeCodecOption(opts.Profile)
	if err != nil {
		s.Fatal("Failed to get codec option: ", err)
	}

	testArgs := []string{logging.ChromeVmoduleFlag(),
		fmt.Sprintf("--codec=%s", codec),
		yuvPath,
		yuvJSONPath,
	}

	exec := filepath.Join(chrome.BinTestDir, "video_encode_accelerator_tests")
	logfile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%d.txt", filepath.Base(exec), time.Now().Unix()))
	t := gtest.New(exec, gtest.Logfile(logfile),
		gtest.ExtraArgs(testArgs...),
		gtest.UID(int(sysutil.ChronosUID)))

	if report, err := t.Run(ctx); err != nil {
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}
