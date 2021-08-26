// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

const runAlsaConformanceTest string = "run-alsa-conformance-test.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtioCrasSnd,
		Desc:         "Tests that the crosvm CRAS virtio-snd device works correctly",
		Contacts:     []string{"woodychow@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runAlsaConformanceTest},
		Timeout:      20 * time.Minute,
		SoftwareDeps: []string{"vm_host", "dlc"},
		Fixture:      "vmDLC",
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

	success := results.Fail != 0
	return success, fmt.Sprintf("%d passed, %d failed.", results.Pass, results.Fail), nil
}

func VirtioCrasSnd(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	// td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.VirtioCrasSnd.")
	// if err != nil {
	// 	s.Fatal("Failed to create temporary directory: ", err)
	// }
	// defer os.RemoveAll(td)

	// The test needs the execute bit set on every component in the test directory
	// in order for rename(2) as a non-root user to succeed.
	// if err := os.Chmod(td, 0755); err != nil {
	// 	s.Fatal("Failed to change permissions on temporary directory: ", err)
	// }

	data := s.FixtValue().(dlc.FixtData)
	crosvmLogPath := filepath.Join(s.OutDir(), "crosvm.log")
	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")

	playbackResultPath := filepath.Join(s.OutDir(), "playback.json")
	captureResultPath := filepath.Join(s.OutDir(), "capture.json")

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", s.DataPath(runAlsaConformanceTest)),
		"--",
		playbackResultPath,
		captureResultPath,
	}

	// The sandbox needs to be disabled because the test creates some device nodes, which is
	// only possible when running as root in the initial namespace.
	args := []string{
		"run",
		"-p", strings.Join(params, " "),
		"-c", "1",
		"-m", "256",
		// "-s", td,
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", kernelLogPath),
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"--cras-snd",
		"--disable-sandbox",
		data.Kernel,
	}

	output, err := os.Create(crosvmLogPath)
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()

	printLog := func() {
		crosvmLog, err := ioutil.ReadFile(crosvmLogPath)
		if err != nil {
			s.Fatal("Failed to read serial log: ", err)
		}
		s.Log(string(crosvmLog))

		kernelLog, err := ioutil.ReadFile(kernelLogPath)
		if err != nil {
			s.Fatal("Failed to read serial log: ", err)
		}
		s.Log(string(kernelLog))
	}

	s.Log("Running Alsa conformance test")
	cmd := testexec.CommandContext(ctx, "crosvm", args...)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		printLog()
		s.Fatal("Failed to run crosvm: ", err)
	}

	printLog()

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
