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

	"chromiumos/tast/common/testexec"
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
		Desc:         "Tests different audio devices in crosvm with alsa conformance test",
		Contacts:     []string{"woodychow@google.com", "paulhsia@google.com", "chromeos-audio-bugs@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runAlsaConformanceTest},
		Timeout:      8 * time.Minute,
		SoftwareDeps: []string{"vm_host", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{{
			Name: "virtio_cras_snd",
			Val: alsaConfig{
				deviceArgs: []string{"--cras-snd", "capture=true,socket_type=legacy"},
			},
		}, {
			Name: "ac97",
			Val: alsaConfig{
				deviceArgs: []string{"--ac97", "backend=cras,capture=true,socket_type=legacy"},
			},
		}},
	})
}

func parseResults(ctx context.Context, path string) (bool, string, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return false, "", errors.Wrapf(err, "failed to read result file %s", path)
	}

	var results struct {
		Fail int `json:"fail"`
		Pass int `json:"pass"`
	}

	if err := json.Unmarshal(buf, &results); err != nil {
		return false, "", errors.Wrapf(err, "failed to parse json file %s", path)
	}

	success := results.Fail == 0
	return success, fmt.Sprintf("%d passed, %d failed", results.Pass, results.Fail), nil
}

func AudioAlsaConformance(ctx context.Context, s *testing.State) {
	config := s.Param().(alsaConfig)
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

	cmd, err := audioutils.CrosvmCmd(ctx, data.Kernel, kernelLogPath, kernelArgs, config.deviceArgs)
	if err != nil {
		s.Fatal("Failed to get crosvm cmd: ", err)
	}

	s.Log("Running Alsa conformance test")
	if err = cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	playbackSuccess, playbackStr, err := parseResults(ctx, playbackResultPath)
	if err != nil {
		s.Fatal("Failed to parse playback results: ", err)
	}
	captureSuccess, captureStr, err := parseResults(ctx, captureResultPath)
	if err != nil {
		s.Fatal("Failed to parse capture results: ", err)
	}

	combinedStr := fmt.Sprintf("Playback: %s. Capture: %s", playbackStr, captureStr)
	if !playbackSuccess || !captureSuccess {
		s.Fatal("Test failed. " + combinedStr)
	}
	s.Log(combinedStr)

}
