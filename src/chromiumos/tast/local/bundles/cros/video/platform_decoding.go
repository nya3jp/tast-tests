// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

// commandBuilderFn is the function type to generate the command line with arguments.
type commandBuilderDecodeFn func(exe, filename string) (command []string)

type platformDecodingParams struct {
	filename       string
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
		Params: []testing.Param{
			{
				Name: "vaapi_vp9",
				Val: platformDecodingParams{
					filename:       "resolution_change_500frames.vp9.ivf",
					decoder:        filepath.Join(chrome.BinTestDir, "decode_test"),
					commandBuilder: vp9decodeVAAPIargs,
				},
				ExtraSoftwareDeps: []string{"vaapi", caps.HWDecodeVP9},
				ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
			}, {
				Name: "v4l2_vp9",
				Val: platformDecodingParams{
					filename:       "1080p_30fps_300frames.vp9.ivf",
					decoder:        "v4l2_stateful_decoder",
					commandBuilder: vp9decodeV4L2args,
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("trogdor")),
				ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWDecodeVP9},
				// TODO(b/180615056): need Dynamic Resolution Change support to use resolution_change_500frames.vp9.ivf like vaapi
				ExtraData: []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
			},
		},
	})
}

// readMetadata reads metadata from metadata json.
func readMetadata(metadataPath string) (meta map[string]interface{}, err error) {
	metadataJSONBytes, err := ioutil.ReadFile(metadataPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read metadata file at %s", metadataPath)
	}

	if err = json.Unmarshal(metadataJSONBytes, &meta); err != nil {
		return nil, errors.Wrapf(err, "failed to read json from metadata file at %s", metadataPath)
	}

	return
}

// verifyContent compares expected per-frame hashes from metadata json to actual
// hashes.
func verifyContent(expectedHashesPath, actualOutput string) error {
	meta, err := readMetadata(expectedHashesPath)
	if err != nil {
		return errors.Wrap(err, "failed to verify per-frame hashes")
	}
	expected, ok := meta["md5_checksums"].([]interface{})
	if !ok {
		return errors.Errorf("`md5_checksums` in metadata at %s not a slice; got %v", expectedHashesPath, meta["md5_checksums"])
	}

	actual := strings.Split(strings.TrimSpace(actualOutput), "\n")
	if len(expected) != len(actual) {
		return errors.Errorf("expected and actual number of frames mismatched (%d != %d)", len(expected), len(actual))
	}

	var first string
	var count int
	for i, ex := range expected {
		if _, ok := ex.(string); !ok {
			return errors.Errorf("failed to cast expected hash %v of type %T to string", ex, ex)
		}
		if wanted, got := strings.TrimSpace(ex.(string)), strings.TrimSpace(actual[i]); wanted != got {
			count++
			if first == "" {
				first = fmt.Sprintf("frame %d (%s != %s)", i, wanted, got)
			}
		}
	}

	if count > 0 {
		return errors.Errorf("%d mismatched hashes, first at %s", count, first)
	}

	return nil
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
	stdout, stderr, err := testexec.CommandContext(
		ctx, command[0], command[1:]...,
	).SeparatedOutput(testexec.DumpLogOnError)

	if err != nil {
		output := append(stdout, stderr...)
		s.Fatalf("%v failed unexpectedly: %v", exec, errors.Wrap(err, string(output)))
	}

	if err := verifyContent(s.DataPath(testOpt.filename+".json"), string(stdout)); err != nil {
		s.Fatalf("%v failed to verify content: %v", exec, errors.Wrap(err, testOpt.filename))
	}
}

// vp9decodeV4L2args constructs the command line for the VP9 decoding binary exe for v4l2.
func vp9decodeV4L2args(exe, filename string) []string {
	return []string{exe, "--file=" + filename}
}

// vp9decodeVAAPIargs constructs the command line for the VP9 decoding binary exe for vaapi.
func vp9decodeVAAPIargs(exe, filename string) []string {
	return []string{
		exe,
		"--video=" + filename,
		"--md5",
		// vpxdec is used to compute reference hashes, and outputs only those for
		// visible frames
		"--visible",
	}
}
