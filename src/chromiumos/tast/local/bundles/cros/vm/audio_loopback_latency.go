// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/vm/audioutils"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

const (
	runLoopbackLatency string = "run-loopback-latency.sh"
	loop               int    = 5
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioLoopbackLatency,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures loopback latency of different audio devices in crosvm",
		Contacts:     []string{"paulhsia@google.com", "normanbt@chromium.org", "chromeos-audio-bugs@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{runLoopbackLatency},
		Timeout:      8 * time.Minute,
		SoftwareDeps: []string{"vm_host", "chrome", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{{
			Name: "virtio_cras_snd",
			Val: audioutils.Config{
				CrosvmArgs: []string{"--virtio-snd", "capture=true,backend=cras,socket_type=legacy"},
			},
		}, {
			Name: "vhost_user_cras",
			Val: audioutils.Config{
				VhostUserArgs: []string{"snd", "--config", "capture=true,backend=cras,socket_type=legacy"},
			},
		}, {
			Name: "ac97",
			Val: audioutils.Config{
				CrosvmArgs: []string{"--ac97", "backend=cras,capture=true,socket_type=legacy"},
			},
		}},
	})
}

func AudioLoopbackLatency(ctx context.Context, s *testing.State) {
	data := s.FixtValue().(dlc.FixtData)
	bufferSizes := []string{"512", "1024", "2048", "4096", "8192"}

	config := s.Param().(audioutils.Config)

	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}
	defer unload(ctx)

	if err = audio.SetupLoopback(ctx); err != nil {
		s.Fatal("Failed to setup loopback device: ", err)
	}

	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	loopbackLogPath := filepath.Join(s.OutDir(), "loopback.txt")

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runLoopbackLatency)),
		"--",
		strconv.Itoa(loop),
		loopbackLogPath,
	}
	kernelArgs = append(kernelArgs, bufferSizes...)

	if err := audioutils.RunCrosvm(ctx, data.Kernel, kernelLogPath, kernelArgs, config); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	perfValues := perf.NewValues()
	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()

	for _, bufferSize := range bufferSizes {
		latencyResult, err := audio.ParseLoopbackLatencyResult(loopbackLogPath+"."+bufferSize, loop)
		if err != nil {
			s.Fatal("Failed to read loopback log: ", err)
		}

		audio.UpdatePerfValuesFromResult(perfValues, latencyResult, bufferSize)
		if latencyResult.ExpectedLoops != latencyResult.GetNumOfValidLoops() {
			s.Logf(
				"Requested %d loops, got %d for bufferSize=%s",
				loop, latencyResult.GetNumOfValidLoops(), bufferSize,
			)
			return
		}
	}
}
