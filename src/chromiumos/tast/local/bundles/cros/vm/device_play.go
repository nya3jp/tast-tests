// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/vm/audioutils"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

const runDevicePlay string = "run-device-play.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         DevicePlay,
		Desc:         "Checks that the sound device is recognized in crosvm with aplay list",
		Contacts:     []string{"pteerapong@google.com", "chromeos-audio-bugs@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{runDevicePlay},
		Timeout:      12 * time.Minute,
		SoftwareDeps: []string{"vm_host", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{{
			Name: "virtio_cras_snd",
			Val: audioutils.Config{
				CrosvmArgs: []string{"--cras-snd", "capture=true,socket_type=legacy"},
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

func DevicePlay(ctx context.Context, s *testing.State) {
	config := s.Param().(audioutils.Config)
	data := s.FixtValue().(dlc.FixtData)

	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	outputLogPath := filepath.Join(s.OutDir(), "output.txt")

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runDevicePlay)),
		"--",
		outputLogPath,
	}

	if err := audioutils.RunCrosvm(ctx, data.Kernel, kernelLogPath, kernelArgs, config); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	buf, err := ioutil.ReadFile(outputLogPath)
	if err != nil {
		s.Fatalf("Failed to read output file %q: %v", outputLogPath, err)
	}

	devicesCnt := 0
	for _, line := range strings.Split(string(buf), "\n") {
		if strings.HasPrefix(line, "card") {
			devicesCnt++
		}
	}

	if devicesCnt != 1 {
		s.Errorf("Devices count incorrect: got %v, want 1", devicesCnt)
	}
}
