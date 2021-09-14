// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

const runLoopbackLatency string = "run-loopback-latency.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioLoopbackLatency,
		Desc:         "Measures the loopback latency of different audio devices in crosvm",
		Contacts:     []string{"woodychow@google.com", "paulhsia@google.com", "chromeos-audio-bugs@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runLoopbackLatency},
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

func AudioLoopbackLatency(ctx context.Context, s *testing.State) {
	config := s.Param().(alsaConfig)

	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}
	defer unload(ctx)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create cras: ", err)
	}

	audio_nodes, err := cras.GetNodes(ctx)
	if err != nil {
		s.Fatal("Failed to get nodes: ", err)
	}

	var playbackDevice, captureDevice audio.CrasNode
	for _, n := range audio_nodes {
		if n.DeviceName == "Loopback Playback" {
			playbackDevice = n
		}
		// Regard the front mic as the internal mic.
		if n.DeviceName == "Loopback Capture" {
			captureDevice = n
		}
	}

	cras.SetActiveNode(ctx, playbackDevice)
	cras.SetActiveNode(ctx, captureDevice)

	data := s.FixtValue().(dlc.FixtData)
	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")

	loopbackLogPath := filepath.Join(s.OutDir(), "loopback.txt")

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", s.DataPath(runLoopbackLatency)),
		"--",
		loopbackLogPath,
	}

	args := []string{"run"}
	args = append(args, config.deviceArgs...)
	args = append(args,
		"-p", strings.Join(params, " "),
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", kernelLogPath),
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		data.Kernel)

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
	kernelLog, err := ioutil.ReadFile(kernelLogPath)
	if err != nil {
		s.Fatal("Failed to read kernel log: ", err)
	}
	s.Log(string(kernelLog))

	if err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}
}
