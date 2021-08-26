// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
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

	args := []string{"run"}
	args = append(args, config.deviceArgs...)
	args = append(args,
		"-p", strings.Join(params, " "),
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", kernelLogPath),
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		data.Kernel)

	s.Log("Running Alsa conformance test")
	cmd := testexec.CommandContext(ctx, "crosvm", args...)

	// Same effect as calling `newgrp cras` before `crosvm` in shell
	// This is needed to access /run/cras/.cras_socket (legacy socket)
	crasGrp, err := user.LookupGroup("cras")
	if err != nil {
		s.Fatal("Failed to find group id for cras: ", err)
	}
	crasGrpID, err := strconv.ParseUint(crasGrp.Gid, 10, 32)
	if err != nil {
		s.Fatal("Failed to convert cras grp id to integer: ", err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         0,
			Gid:         0,
			Groups:      []uint32{uint32(crasGrpID)},
			NoSetGroups: false,
		},
	}

	crosvmLog, err := cmd.Output(testexec.DumpLogOnError)
	s.Log(string(crosvmLog))

	kernelLog, readErr := ioutil.ReadFile(kernelLogPath)
	if readErr != nil {
		s.Fatal("Failed to read kernel log: ", err)
	}
	s.Log(string(kernelLog))

	if err != nil {
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
