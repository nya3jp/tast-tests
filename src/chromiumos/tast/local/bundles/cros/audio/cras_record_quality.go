// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// TODO(b/244254621) : remove "sasukette" when b/244254621 is fixed.
// TODO(b/256522810) : remove "steelix" when b/256522810 is fixed.
// TODO(b/256523120) : remove "gimble" when b/256523120 is fixed.
// TODO(b/256534123) : remove "treeya360" when b/256534123 is fixed.
var crasRecordQualityUnstableModels = []string{"sasukette", "steelix", "gimble", "treeya360"}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrasRecordQuality,
		Desc:         "Verifies recorded samples from CRAS are correct",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
		Attr:         []string{"group:mainline"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"audio_stable"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(crasRecordQualityUnstableModels...)),
		}, {
			Name:              "unstable_platform",
			ExtraSoftwareDeps: []string{"audio_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "unstable_model",
			ExtraHardwareDeps: hwdep.D(hwdep.Model(crasRecordQualityUnstableModels...)),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func CrasRecordQuality(ctx context.Context, s *testing.State) {
	const duration = 2 * time.Second

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Stop UI in advance for this test to avoid the node being selected by UI.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(cleanupCtx, "ui")

	defer func(ctx context.Context) {
		if s.HasError() {
			if err := crastestclient.DumpAudioDiagnostics(ctx, s.OutDir()); err != nil {
				s.Error("Failed to dump audio diagnostics: ", err)
			}
		}
	}(cleanupCtx)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to CRAS: ", err)
	}

	if err := cras.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
		s.Fatal("Failed to set internal mic active: ", err)
	}

	// Set timeout to duration + 5s, which is the time buffer to complete the normal execution.
	runCtx, cancel := context.WithTimeout(ctx, duration+5*time.Second)
	defer cancel()

	rawFile := filepath.Join(s.OutDir(), "recorded.raw")
	wavFile := filepath.Join(s.OutDir(), "recorded.wav")
	clippedFile := filepath.Join(s.OutDir(), "clipped.wav")

	// Record function by CRAS.
	if err := testexec.CommandContext(
		runCtx, "cras_test_client",
		"--capture_file", rawFile,
		"--duration", strconv.FormatFloat(duration.Seconds(), 'f', -1, 64),
		"--num_channels", "2",
		"--rate", "48000",
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to record from CRAS: ", err)
	}

	if err := audio.ConvertRawToWav(ctx, rawFile, wavFile, 48000, 2); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}

	// Remove first 0.5 seconds to avoid pop noise.
	if err := audio.TrimFileFrom(ctx, wavFile, clippedFile, 500*time.Millisecond); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}

	if err := audio.CheckRecordingNotZero(ctx, clippedFile); err != nil {
		s.Error("Failed to check quality: ", err)
	}
}
