// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/vm/audioutils"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

const runAudioArecord string = "run-arecord.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioArecord,
		Desc:         "Check that capture devices are listed correctly",
		Contacts:     []string{"normanbt@google.com", "chromeos-audio-bugs@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runAudioArecord},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"vm_host", "dlc"},
		Fixture:      "vmDLC",
	})
}

func AudioArecord(ctx context.Context, s *testing.State) {
	data := s.FixtValue().(dlc.FixtData)

	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	outputLogPath := filepath.Join(s.OutDir(), "output.txt")

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runAudioArecord)),
		"--",
		outputLogPath,
	}

	deviceRegex := regexp.MustCompile(`card \d+:.*\[(?P<Card>.*)\], device \d+:.*\[(?P<Device>.*)\]`)

	for _, tc := range []struct {
		name                     string
		crosvmArgs               []string
		vhostUserArgs            []string
		expectedCardNames        []string
		expectedDeviceNames      []string
		expectedStreamsPerDevice int
	}{
		{
			name:                     "virtio_cras_snd",
			crosvmArgs:               []string{"--cras-snd", "capture=true,socket_type=legacy"},
			expectedCardNames:        []string{"VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0"},
			expectedStreamsPerDevice: 1,
		},
		{
			name:                     "virtio_cras_snd_3_devices_4_streams",
			crosvmArgs:               []string{"--cras-snd", "capture=true,socket_type=legacy,num_input_devices=3,num_input_streams=4"},
			expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
			expectedStreamsPerDevice: 4,
		},
		{
			name:                     "virtio_cras_snd_1_device_3_streams",
			crosvmArgs:               []string{"--cras-snd", "capture=true,socket_type=legacy,num_input_streams=3"},
			expectedCardNames:        []string{"VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0"},
			expectedStreamsPerDevice: 3,
		},
		{
			name:                     "virtio_cras_snd_3_devices_1_stream",
			crosvmArgs:               []string{"--cras-snd", "capture=true,socket_type=legacy,num_input_devices=3"},
			expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
			expectedStreamsPerDevice: 1,
		},

		{
			name:                     "vhost_user_cras",
			vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy"},
			expectedCardNames:        []string{"VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0"},
			expectedStreamsPerDevice: 1,
		},
		{
			name:                     "vhost_user_cras_3_devices_4_streams",
			vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy,num_input_devices=3,num_input_streams=4"},
			expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
			expectedStreamsPerDevice: 4,
		},
		{
			name:                     "vhost_user_cras_1_device_3_streams",
			vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy,num_input_streams=3"},
			expectedCardNames:        []string{"VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0"},
			expectedStreamsPerDevice: 3,
		},
		{
			name:                     "vhost_user_cras_3_devices_1_stream",
			vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy,num_input_devices=3"},
			expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
			expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
			expectedStreamsPerDevice: 1,
		},

		{
			name:                     "ac97",
			crosvmArgs:               []string{"--ac97", "backend=cras,socket_type=legacy"},
			expectedCardNames:        []string{"Intel 82801AA-ICH", "Intel 82801AA-ICH"},
			expectedDeviceNames:      []string{"Intel 82801AA-ICH", "Intel 82801AA-ICH - MIC ADC"},
			expectedStreamsPerDevice: 1,
		},
	} {
		config := audioutils.Config{
			CrosvmArgs:    tc.crosvmArgs,
			VhostUserArgs: tc.vhostUserArgs,
		}
		if err := audioutils.RunCrosvm(ctx, data.Kernel, kernelLogPath, kernelArgs, config); err != nil {
			s.Fatal("Failed to run crosvm: ", err)
		}

		output, err := ioutil.ReadFile(outputLogPath)
		if err != nil {
			s.Fatalf("Failed to read output file %q: %v", outputLogPath, err)
		}

		testing.ContextLog(ctx, string(output))
		lines := strings.Split(string(output), "\n")
		devicesCnt := 0
		for idx := 0; idx < len(lines); idx++ {
			lines[idx] = strings.TrimSpace(lines[idx])

			match := deviceRegex.FindStringSubmatch(lines[idx])
			if match == nil {
				continue
			}

			if devicesCnt >= len(tc.expectedCardNames) {
				s.Errorf("%s card name count is more than expected: got %s", tc.name, match[1])
				continue
			}

			if devicesCnt >= len(tc.expectedDeviceNames) {
				s.Errorf("%s device name count is more than expected: got %s", tc.name, match[2])
				continue
			}

			if match[1] != tc.expectedCardNames[devicesCnt] {
				s.Errorf("%s card name incorrect: got %s, want %s", tc.name, match[1], tc.expectedCardNames[devicesCnt])
			}
			if match[2] != tc.expectedDeviceNames[devicesCnt] {
				s.Errorf("%s device name incorrect: got %s, want %s", tc.name, match[2], tc.expectedDeviceNames[devicesCnt])
			}
			devicesCnt++

			// Expect next line: "Subdevices: n/n"
			idx++
			if idx >= len(lines) {
				s.Errorf("%s device %s has no subdevices line after it", tc.name, match[2])
				break
			}

			lines[idx] = strings.TrimSpace(lines[idx])
			expectSubdevices := fmt.Sprintf("Subdevices: %d/%d", tc.expectedStreamsPerDevice, tc.expectedStreamsPerDevice)
			if lines[idx] != expectSubdevices {
				s.Errorf("%s device %s subdevice line incorrect: got %q, want %q", tc.name, match[2], lines[idx], expectSubdevices)
			}
		}

		if devicesCnt != len(tc.expectedDeviceNames) {
			s.Errorf("%s device count incorrect: got %d, want %d", tc.name, devicesCnt, len(tc.expectedDeviceNames))
		}
	}

}
