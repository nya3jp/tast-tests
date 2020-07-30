// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package encode provides common code to run Chrome binary tests for video encoding.
package encode

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
	JSONName string
	Profile  videotype.CodecProfile
}

func codecProfileToEncodeCodecOption(profile videotype.CodecProfile) (string, error) {
	if profile == videotype.H264Prof {
		return "h264baseline", nil
	}
	if profile == videotype.VP8Prof {
		return "vp8", nil
	}
	if profile == videotype.VP9Prof {
		return "vp9", nil
	}
	return "", errors.Errorf("unknown codec profile: %s", profile)
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err

}

// RunNewAccelVideoTest runs all tests in video_encode_accelerator_tests.
func RunNewAccelVideoTest(ctx context.Context, s *testing.State, opts TestOptionsNew) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := upstart.StopJob(shortCtx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	yuvPath, err := encoding.PrepareYUV(shortCtx, s.DataPath(opts.WebMName), videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(yuvPath)
	yuvJSONPath := yuvPath + ".json"
	if err := copyFile(s.DataPath(opts.JSONName), yuvJSONPath); err != nil {
		s.Fatal("Failed to copy json file: ", s.DataPath(opts.JSONName), yuvJSONPath)
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
