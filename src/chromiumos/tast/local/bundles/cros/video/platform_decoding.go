// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type failExpectedFn func(output []byte) bool

type platformDecodingParams struct {
	filename     string
	failExpected failExpectedFn
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformDecoding,
		Desc: "Smoke tests libva decoding by running the media/gpu/vaapi/test:decode_test binary",
		Contacts: []string{
			"jchinlee@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"vaapi"},
		Params: []testing.Param{{
			Name: "vp9",
			Val: platformDecodingParams{
				filename:     "resolution_change_500frames.vp9.ivf",
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			// Attempt to decode an unsupported codec to ensure that the binary is not
			// unconditionally succeeding, i.e. not crashing even when expected to.
			Name: "unsupported_codec_fail",
			Val: platformDecodingParams{
				filename: "resolution_change_500frames.vp8.ivf",
				failExpected: func(output []byte) bool {
					return strings.Contains(string(output), "Codec VP80 not supported.")
				},
			},
			ExtraData: []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}},
	})
}

// PlatformDecoding runs the media/gpu/vaapi/test:decode_test binary on the
// file specified in the testing state. The test fails if any of the VAAPI calls
// fail (or if the test is incorrectly invoked): notably, the binary does not
// check for correctness of decoded output. This test is motivated by instances
// in which libva uprevs may introduce regressions and cause decoding to break
// for reasons unrelated to Chrome.
func PlatformDecoding(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(platformDecodingParams)
	const cleanupTime = 90 * time.Second

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to create new video logger: ", err)
	}
	defer vl.Close()

	// Only a single process can have access to the GPU. We do not strictly need
	// to `stop ui` to run the binary, but still do so to shut down the browser
	// and improve benchmarking accuracy.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(cleanupCtx, "ui")

	// Run the decode_test binary, propagating its errors: the decode_test binary
	// fails if the VAAPI calls themselves error, the binary is called on
	// unsupported inputs or could not open the DRI render node, or the binary
	// otherwise crashes.
	const exec = "decode_test"
	testing.ContextLog(ctx, "Running ", exec)
	output, err := testexec.CommandContext(
		ctx,
		filepath.Join(chrome.BinTestDir, exec),
		"--video="+s.DataPath(testOpt.filename),
	).CombinedOutput(testexec.DumpLogOnError)

	if err != nil && (testOpt.failExpected == nil || !testOpt.failExpected(output)) {
		s.Fatalf("%v failed unexpectedly: %v", exec, errors.Wrap(err, string(output)))
	}
	if err == nil && testOpt.failExpected != nil {
		s.Fatalf("%v passed when expected to fail", exec)
	}
}
