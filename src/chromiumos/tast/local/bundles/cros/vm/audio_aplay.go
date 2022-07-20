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

const runAudioAplay string = "run-aplay.sh"

type audioAplayParams struct {
	crosvmArgs               []string
	vhostUserArgs            []string
	expectedCardNames        []string
	expectedDeviceNames      []string
	expectedStreamsPerDevice int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioAplay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that playback devices are listed correctly",
		Contacts:     []string{"pteerapong@google.com", "chromeos-audio-bugs@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runAudioAplay},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"vm_host", "chrome", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{
			{
				Name: "virtio_null_snd",
				Val: audioAplayParams{
					crosvmArgs:               []string{"--virtio-snd", "capture=true,backend=null"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 1,
				},
			},
			{
				Name: "virtio_cras_snd",
				Val: audioAplayParams{
					crosvmArgs:               []string{"--virtio-snd", "capture=true,backend=cras,socket_type=legacy"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 1,
				},
			},
			{
				Name: "virtio_cras_snd_3_devices_4_streams",
				Val: audioAplayParams{
					crosvmArgs:               []string{"--virtio-snd", "capture=true,backend=cras,socket_type=legacy,num_output_devices=3,num_output_streams=4"},
					expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
					expectedStreamsPerDevice: 4,
				},
			},
			{
				Name: "virtio_cras_snd_1_device_3_streams",
				Val: audioAplayParams{
					crosvmArgs:               []string{"--virtio-snd", "capture=true,backend=cras,socket_type=legacy,num_output_streams=3"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 3,
				},
			},
			{
				Name: "virtio_cras_snd_3_devices_1_stream",
				Val: audioAplayParams{
					crosvmArgs:               []string{"--virtio-snd", "capture=true,backend=cras,socket_type=legacy,num_output_devices=3"},
					expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
					expectedStreamsPerDevice: 1,
				},
			},

			{
				Name: "vhost_user_cras",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 1,
				},
			},
			{
				Name: "vhost_user_cras_3_devices_4_streams",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy,num_output_devices=3,num_output_streams=4"},
					expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
					expectedStreamsPerDevice: 4,
				},
			},
			{
				Name: "vhost_user_cras_1_device_3_streams",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy,num_output_streams=3"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 3,
				},
			},
			{
				Name: "vhost_user_cras_3_devices_1_stream",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"cras-snd", "--config", "capture=true,socket_type=legacy,num_output_devices=3"},
					expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
					expectedStreamsPerDevice: 1,
				},
			},

			{
				Name: "ac97",
				Val: audioAplayParams{
					crosvmArgs:               []string{"--ac97", "backend=cras,socket_type=legacy"},
					expectedCardNames:        []string{"Intel 82801AA-ICH"},
					expectedDeviceNames:      []string{"Intel 82801AA-ICH"},
					expectedStreamsPerDevice: 1,
				},
			},
		},
	})
}

func AudioAplay(ctx context.Context, s *testing.State) {
	param := s.Param().(audioAplayParams)
	data := s.FixtValue().(dlc.FixtData)

	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	outputLogPath := filepath.Join(s.OutDir(), "output.txt")

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runAudioAplay)),
		"--",
		outputLogPath,
	}

	config := audioutils.Config{
		CrosvmArgs:    param.crosvmArgs,
		VhostUserArgs: param.vhostUserArgs,
	}

	// Example of the output from `aplay -l` when using 3 devices, 4 streams
	/*
		**** List of PLAYBACK Hardware Devices ****
		card 0: SoundCard [VirtIO SoundCard], device 0: virtio-snd [VirtIO PCM 0]
		  Subdevices: 4/4
		  Subdevice #0: subdevice #0
		  Subdevice #1: subdevice #1
		  Subdevice #2: subdevice #2
		  Subdevice #3: subdevice #3
		card 0: SoundCard [VirtIO SoundCard], device 1: virtio-snd [VirtIO PCM 1]
		  Subdevices: 4/4
		  Subdevice #0: subdevice #0
		  Subdevice #1: subdevice #1
		  Subdevice #2: subdevice #2
		  Subdevice #3: subdevice #3
		card 0: SoundCard [VirtIO SoundCard], device 2: virtio-snd [VirtIO PCM 2]
		  Subdevices: 4/4
		  Subdevice #0: subdevice #0
		  Subdevice #1: subdevice #1
		  Subdevice #2: subdevice #2
		  Subdevice #3: subdevice #3
	*/

	deviceRegex := regexp.MustCompile(`card \d+:.*\[(?P<Card>.*)\], device \d+:.*\[(?P<Device>.*)\]`)

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

		if devicesCnt >= len(param.expectedCardNames) {
			s.Errorf("card name count is more than expected: got %s", match[1])
			continue
		}

		if devicesCnt >= len(param.expectedDeviceNames) {
			s.Errorf("device name count is more than expected: got %s", match[2])
			continue
		}

		if match[1] != param.expectedCardNames[devicesCnt] {
			s.Errorf("card name incorrect: got %s, want %s", match[1], param.expectedCardNames[devicesCnt])
		}
		if match[2] != param.expectedDeviceNames[devicesCnt] {
			s.Errorf("device name incorrect: got %s, want %s", match[2], param.expectedDeviceNames[devicesCnt])
		}
		devicesCnt++

		// Expect next line: "Subdevices: n/n"
		idx++
		if idx >= len(lines) {
			s.Errorf("device %s has no subdevices line after it", match[2])
			break
		}

		lines[idx] = strings.TrimSpace(lines[idx])
		expectSubdevices := fmt.Sprintf("Subdevices: %d/%d", param.expectedStreamsPerDevice, param.expectedStreamsPerDevice)
		if lines[idx] != expectSubdevices {
			s.Errorf("device %s subdevices line incorrect: got %q, want %q", match[2], lines[idx], expectSubdevices)
		}
	}

	if devicesCnt != len(param.expectedDeviceNames) {
		s.Errorf("device count incorrect: got %d, want %d", devicesCnt, len(param.expectedDeviceNames))
	}

}
