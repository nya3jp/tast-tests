// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for video decoding.

package decode

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// RunVaapiSanityTest runs the media/gpu/vaapi/test:decode_test binary on
// the specified file. The test fails if any of the VAAPI calls fail (or if
// the test is incorrectly invoked): notably, the binary does not check for
// correctness of decoded otuput. This test is motivated by instances in
// which libva uprevs may introduce regressions and cause decoding to break
// for reasons unrelated to Chrome.
func RunVaapiSanityTest(ctx context.Context, s *testing.State, filename string) {
	const cleanupTime = 10 * time.Second

	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Only a single process can have access to the GPU, so we are required to
	// call "stop ui" at the start of the test. This will shut down the chrome
	// process and allow us to claim ownership of the GPU.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Run the decode_test binary, propagating its errors: the decode_test binary
	// only fails if the VAAPI calls themselves error.
	const exec = "decode_test"
	testing.ContextLogf(ctx, "Running %v", exec)
	if output, err := testexec.CommandContext(
		ctx,
		filepath.Join(chrome.BinTestDir, exec),
		"--video="+s.DataPath(filename),
	).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to run %v: %v\n%v", exec, err, string(output))
	}
	testing.ContextLog(ctx, "No failures detected, running libva decode sanity test successful")
}
