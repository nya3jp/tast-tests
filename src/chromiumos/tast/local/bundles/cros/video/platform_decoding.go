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
	"chromiumos/tast/testing/hwdep"
)

type failExpectedFn func(output []byte) bool

// commandBuilderFn is the function type to generate the command line with arguments.
type commandBuilderDecodeFn func(exe, filename string) (command []string)

type platformDecodingParams struct {
	filename       string
	failExpected   failExpectedFn
	decoder        string                 // command line decoder binary
	commandBuilder commandBuilderDecodeFn // Function to create the command line arguments.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformDecoding,
		Desc: "Smoke tests for vaapi libva decoding by running the media/gpu/vaapi/test:decode_test binary, for v4l2 decoding by running the drm-tests/v4l2_decode binary",
		Contacts: []string{
			"jchinlee@chromium.org",
			"stevecho@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name: "vaapi_vp9",
			Val: platformDecodingParams{
				filename:       "resolution_change_500frames.vp9.ivf",
				failExpected:   nil,
				decoder:        filepath.Join(chrome.BinTestDir, "decode_test"),
				commandBuilder: vp9decodeVAAPIargs,
			},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWDecodeVP9},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			Name: "v4l2_vp9",
			Val: platformDecodingParams{
				filename:       "1080p_30fps_300frames.vp9.ivf",
				failExpected:   nil,
				decoder:        "v4l2_stateful_decoder",
				commandBuilder: vp9decodeV4L2args,
			},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("trogdor")),
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWDecodeVP9},
			// TODO(b/180615056): need Dynamic Resolution Change support to use resolution_change_500frames.vp9.ivf like vaapi
			ExtraData: []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
		}, {
			// Attempt to decode an unsupported codec to ensure that the binary is not
			// unconditionally succeeding, i.e. not crashing even when expected to.
			Name: "unsupported_codec_fail",
			Val: platformDecodingParams{
				filename: "resolution_change_500frames.vp8.ivf",
				failExpected: func(output []byte) bool {
					return strings.Contains(string(output), "Codec VP80 not supported.")
				},
				decoder:        filepath.Join(chrome.BinTestDir, "decode_test"),
				commandBuilder: vp9decodeVAAPIargs,
			},
			ExtraSoftwareDeps: []string{"vaapi"},
			ExtraData:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}},
	})
}

// PlatformDecoding runs the media/gpu/vaapi/test:decode_test binary for vaapi
// or drm-tests/v4l2_stateful_decoder binary on the file specified in the testing state.
// The test fails if any of the VAAPI or V4L2 calls fail (or if the test is incorrectly invoked):
// notably, the binary does not check for correctness of decoded output.
// This test is motivated by instances in which libva uprevs may introduce regressions
// and cause decoding to break for reasons unrelated to Chrome.
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

	// Run the decode_test binary for vaapi or the v4l2_stateful_decoder binary
	// for v4l2, propagating its errors: the binary fails if the VAAPI or V4l2 calls
	// themselves error, the binary is called on unsupported inputs or could not open
	// the DRI render node, or the binary otherwise crashes.
	exec := testOpt.decoder
	command := testOpt.commandBuilder(exec, s.DataPath(testOpt.filename))
	testing.ContextLog(ctx, "Running ", exec)
	output, err := testexec.CommandContext(
		ctx,
		command[0],
		command[1],
	).CombinedOutput(testexec.DumpLogOnError)

	if err != nil && (testOpt.failExpected == nil || !testOpt.failExpected(output)) {
		s.Fatalf("%v failed unexpectedly: %v", exec, errors.Wrap(err, string(output)))
	}
	if err == nil && testOpt.failExpected != nil {
		s.Fatalf("%v passed when expected to fail", exec)
	}
}

// vp9decodeV4L2args constructs the command line for the VP9 decoding binary exe for v4l2.
func vp9decodeV4L2args(exe, filename string) (command []string) {
	command = append(command, exe, "--file="+filename)

	return
}

// vp9decodeVAAPIargs constructs the command line for the VP9 decoding binary exe for vaapi.
func vp9decodeVAAPIargs(exe, filename string) (command []string) {
	command = append(command, exe, "--video="+filename)

	return
}
