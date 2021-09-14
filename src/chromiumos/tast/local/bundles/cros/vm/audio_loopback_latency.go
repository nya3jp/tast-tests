// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/vm/audioutils"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/testing"
)

const (
	runLoopbackLatency string = "run-loopback-latency.sh"
	deviceType         string = "ALSA_LOOPBACK"
	loop               int    = 5
	periodSize         int    = 4096
	bufferSize         int    = 8192
)

type loopbackLatencyConfig struct {
	deviceArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioLoopbackLatency,
		Desc:         "Measures loopback latency of different audio devices in crosvm",
		Contacts:     []string{"woodychow@google.com", "paulhsia@google.com", "chromeos-audio-bugs@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runLoopbackLatency},
		Timeout:      8 * time.Minute,
		SoftwareDeps: []string{"vm_host", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{{
			Name: "virtio_cras_snd",
			Val: loopbackLatencyConfig{
				deviceArgs: []string{"--cras-snd", "capture=true,socket_type=legacy"},
			},
		}, {
			Name: "ac97",
			Val: loopbackLatencyConfig{
				deviceArgs: []string{"--ac97", "backend=cras,capture=true,socket_type=legacy"},
			},
		}},
	})
}

func findDevice(ctx context.Context, devices []audio.CrasNode, isInput bool) (audio.CrasNode, error) {
	for _, n := range devices {
		if n.Type == deviceType && n.IsInput == isInput {
			return n, nil
		}
	}
	return audio.CrasNode{}, errors.Errorf("cannot find device with type=%s and isInput=%v", deviceType, isInput)
}

func setupLoopback(ctx context.Context) error {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cras")
	}

	var playbackFound, captureFound bool
	checkLoopbackNode := func(n *audio.CrasNode) bool {
		if n.Type != deviceType {
			return false
		}
		if n.IsInput {
			captureFound = true
		} else {
			playbackFound = true
		}
		return captureFound && playbackFound
	}

	err = cras.WaitForDeviceUntil(ctx, checkLoopbackNode, 5*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to wait for loopback devices")
	}

	audioDevices, err := cras.GetNodes(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get nodes")
	}

	playbackDevice, err := findDevice(ctx, audioDevices, false)
	if err != nil {
		return errors.Wrap(err, "failed to find audio device")
	}

	captureDevice, err := findDevice(ctx, audioDevices, true)
	if err != nil {
		return errors.Wrap(err, "failed to find audio device")
	}

	cras.SetActiveNode(ctx, playbackDevice)
	cras.SetActiveNode(ctx, captureDevice)
	cras.SetOutputNodeVolume(ctx, playbackDevice, 100)

	return nil
}

func extractNumbers(strs []string) ([]float64, error) {
	extractRe := regexp.MustCompile("[0-9]+")
	var nums []float64
	for _, numberStr := range strs {
		num, err := strconv.Atoi(extractRe.FindString(numberStr))
		if err != nil {
			return []float64{}, errors.Wrap(err, "atoi failed")
		}
		nums = append(nums, float64(num))
	}
	return nums, nil
}

func AudioLoopbackLatency(ctx context.Context, s *testing.State) {
	config := s.Param().(loopbackLatencyConfig)

	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}
	defer unload(ctx)

	if err = setupLoopback(ctx); err != nil {
		s.Fatal("Failed to setup loopback device: ", err)
	}

	data := s.FixtValue().(dlc.FixtData)
	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	loopbackLogPath := filepath.Join(s.OutDir(), "loopback.txt")

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runLoopbackLatency)),
		"--",
		strconv.Itoa(periodSize),
		strconv.Itoa(bufferSize),
		strconv.Itoa(loop),
		loopbackLogPath,
	}

	cmd, err := audioutils.CrosvmCmd(ctx, data.Kernel, kernelLogPath, kernelArgs, config.deviceArgs)
	if err != nil {
		s.Fatal("Failed to get crosvm cmd: ", err)
	}

	if err = cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	loopbackLogBytes, err := ioutil.ReadFile(loopbackLogPath)
	if err != nil {
		s.Fatal("Failed to read loopback log: ", err)
	}
	loopbackLog := string(loopbackLogBytes)

	measuredRe := regexp.MustCompile("Measured Latency: [0-9]+ uS")
	reportedRe := regexp.MustCompile("Reported Latency: [0-9]+ uS")

	measuredLatencies, err := extractNumbers(measuredRe.FindAllString(loopbackLog, -1))
	if err != nil {
		s.Fatal("Extract numbers failed: ", err)
	}
	reportedLatencies, err := extractNumbers(reportedRe.FindAllString(loopbackLog, -1))
	if err != nil {
		s.Fatal("Extract numbers failed: ", err)
	}

	if len(measuredLatencies) != loop || len(reportedLatencies) != loop {
		s.Fatalf("Requested %d loops. Got %d. Increase the buffer/period size?", loop, len(measuredLatencies))
	}

	perfValues := perf.NewValues()
	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()

	perfValues.Set(
		perf.Metric{
			Name:      "measured_latency",
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, measuredLatencies...)
	perfValues.Set(
		perf.Metric{
			Name:      "reported_latency",
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, reportedLatencies...)
}
