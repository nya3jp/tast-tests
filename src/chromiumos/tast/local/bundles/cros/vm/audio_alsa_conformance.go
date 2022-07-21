// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/vm/audioutils"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

type alsaConfig struct {
	deviceArgs []string
}

const runAlsaConformanceTest string = "run-alsa-conformance-test.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioAlsaConformance,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests different audio devices in crosvm with alsa conformance test",
		Contacts:     []string{"paulhsia@google.com", "normanbt@chromium.org", "chromeos-audio-bugs@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{runAlsaConformanceTest},
		Timeout:      12 * time.Minute,
		SoftwareDeps: []string{"vm_host", "chrome", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{{
			Name: "virtio_null_snd",
			Val: audioutils.Config{
				CrosvmArgs: []string{"--virtio-snd", "capture=false,backend=null"},
			},
		}, {
			Name: "virtio_cras_snd",
			Val: audioutils.Config{
				CrosvmArgs: []string{"--virtio-snd", "capture=true,backend=cras,socket_type=legacy"},
			},
		}, {
			Name: "vhost_user_cras",
			Val: audioutils.Config{
				VhostUserArgs: []string{"cras-snd", "--config", "capture=true,socket_type=legacy"},
			},
		}, {
			Name: "ac97",
			Val: audioutils.Config{
				CrosvmArgs: []string{"--ac97", "backend=cras,capture=true,socket_type=legacy"},
			},
		}},
	})
}

func parseResults(ctx context.Context, path string) (int, int, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to read result file %s", path)
	}

	var results struct {
		Fail int `json:"fail"`
		Pass int `json:"pass"`
	}

	if err := json.Unmarshal(buf, &results); err != nil {
		return 0, 0, errors.Wrapf(err, "failed to parse json file %s", path)
	}

	return results.Pass, results.Fail, nil
}

func AudioAlsaConformance(ctx context.Context, s *testing.State) {
	config := s.Param().(audioutils.Config)
	data := s.FixtValue().(dlc.FixtData)

	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	playbackLogPath := filepath.Join(s.OutDir(), "playback.txt")
	playbackResultPath := filepath.Join(s.OutDir(), "playback.json")
	captureLogPath := filepath.Join(s.OutDir(), "capture.txt")
	captureResultPath := filepath.Join(s.OutDir(), "capture.json")

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runAlsaConformanceTest)),
		"--",
		playbackLogPath,
		playbackResultPath,
		captureLogPath,
		captureResultPath,
	}

	if err := audioutils.RunCrosvm(ctx, data.Kernel, kernelLogPath, kernelArgs, config); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	playbackPass, playbackFail, err := parseResults(ctx, playbackResultPath)
	if err != nil {
		s.Fatal("Failed to parse playback results: ", err)
	}
	capturePass, captureFail, err := parseResults(ctx, captureResultPath)
	if err != nil {
		s.Fatal("Failed to parse capture results: ", err)
	}

	s.Logf("Playback: %d passed, %d failed", playbackPass, playbackFail)
	s.Logf("Capture: %d passed, %d failed", capturePass, captureFail)

	perfValues := perf.NewValues()
	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()

	perfValues.Set(
		perf.Metric{
			Name:      "playback_pass",
			Unit:      "n",
			Direction: perf.BiggerIsBetter,
		}, float64(playbackPass))
	perfValues.Set(
		perf.Metric{
			Name:      "playback_fail",
			Unit:      "n",
			Direction: perf.SmallerIsBetter,
		}, float64(playbackFail))
	perfValues.Set(
		perf.Metric{
			Name:      "capture_pass",
			Unit:      "n",
			Direction: perf.BiggerIsBetter,
		}, float64(capturePass))
	perfValues.Set(
		perf.Metric{
			Name:      "capture_fail",
			Unit:      "n",
			Direction: perf.SmallerIsBetter,
		}, float64(captureFail))
}
