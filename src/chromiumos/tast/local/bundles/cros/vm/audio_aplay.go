// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/local/audio"
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
				Name: "vhost_user_null",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"snd", "--config", "capture=true,backend=null,socket_type=legacy"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 1,
				},
			},
			{
				Name: "vhost_user_cras",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"snd", "--config", "capture=true,backend=cras,socket_type=legacy"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 1,
				},
			},
			{
				Name: "vhost_user_cras_3_devices_4_streams",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"snd", "--config", "capture=true,backend=cras,socket_type=legacy,num_output_devices=3,num_output_streams=4"},
					expectedCardNames:        []string{"VirtIO SoundCard", "VirtIO SoundCard", "VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0", "VirtIO PCM 1", "VirtIO PCM 2"},
					expectedStreamsPerDevice: 4,
				},
			},
			{
				Name: "vhost_user_cras_1_device_3_streams",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"snd", "--config", "capture=true,backend=cras,socket_type=legacy,num_output_streams=3"},
					expectedCardNames:        []string{"VirtIO SoundCard"},
					expectedDeviceNames:      []string{"VirtIO PCM 0"},
					expectedStreamsPerDevice: 3,
				},
			},
			{
				Name: "vhost_user_cras_3_devices_1_stream",
				Val: audioAplayParams{
					vhostUserArgs:            []string{"snd", "--config", "capture=true,backend=cras,socket_type=legacy,num_output_devices=3"},
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

	if err := audioutils.RunCrosvm(ctx, data.Kernel, kernelLogPath, kernelArgs, config); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	output, err := ioutil.ReadFile(outputLogPath)
	if err != nil {
		s.Fatalf("Failed to read output file %q: %v", outputLogPath, err)
	}

	if err := audio.CheckAlsaDeviceList(
		ctx, string(output), param.expectedCardNames,
		param.expectedDeviceNames, param.expectedStreamsPerDevice, true,
	); err != nil {
		s.Errorf("Found difference on aplay -l output, err: %s", err)
	}
}
