// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
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
)

type loopbackLatencyConfig struct {
	deviceArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioLoopbackLatency,
		Desc:         "Measures loopback latency of different audio devices in crosvm",
		Contacts:     []string{"woodychow@google.com", "paulhsia@google.com", "chromeos-audio-bugs@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
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

func minAndMax(nums []float64) (float64, float64) {
	min := math.Inf(1)
	max := math.Inf(-1)
	for _, n := range nums {
		max = math.Max(max, n)
		min = math.Min(min, n)
	}
	return min, max
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

	if err = cras.WaitForDeviceUntil(ctx, checkLoopbackNode, 5*time.Second); err != nil {
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
	bufferSizes := []string{"512", "1024", "2048", "4096", "8192"}

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
		strconv.Itoa(loop),
		loopbackLogPath,
	}
	kernelArgs = append(kernelArgs, bufferSizes...)

	cmd, err := audioutils.CrosvmCmd(ctx, data.Kernel, kernelLogPath, kernelArgs, config.deviceArgs)
	if err != nil {
		s.Fatal("Failed to get crosvm cmd: ", err)
	}

	if err = cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	perfValues := perf.NewValues()
	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()

	// Sample output:
	// 	Assign cap_dev hw:0,0
	// 	Assign play_dev hw:0,0
	// 	Found audio
	// 	Played at 1631853475 67453, 8192 delay
	// 	Capture at 1631853475 408149, 4096 delay sample 3904
	// 	Measured Latency: 340696 uS
	// 	Reported Latency: 174666 uS
	measuredRe := regexp.MustCompile("Measured Latency: [0-9]+ uS")
	reportedRe := regexp.MustCompile("Reported Latency: [0-9]+ uS")
	for _, bufferSize := range bufferSizes {
		loopbackLogBytes, err := ioutil.ReadFile(loopbackLogPath + "." + bufferSize)
		if err != nil {
			s.Fatal("Failed to read loopback log: ", err)
		}
		loopbackLog := string(loopbackLogBytes)

		measuredLatencies, err := extractNumbers(measuredRe.FindAllString(loopbackLog, -1))
		if err != nil {
			s.Fatal("Extract numbers failed: ", err)
		}
		reportedLatencies, err := extractNumbers(reportedRe.FindAllString(loopbackLog, -1))
		if err != nil {
			s.Fatal("Extract numbers failed: ", err)
		}

		if len(measuredLatencies) != loop || len(reportedLatencies) != loop {
			s.Logf("Requested %d loops, got %d for bufferSize=%s", loop, len(measuredLatencies), bufferSize)
			continue
		}

		measuredMin, measuredMax := minAndMax(measuredLatencies)
		reportedMin, reportedMax := minAndMax(reportedLatencies)

		perfValues.Set(
			perf.Metric{
				Name:      "measured_latency_" + bufferSize,
				Unit:      "uS",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, measuredLatencies...)
		perfValues.Set(
			perf.Metric{
				Name:      "reported_latency_" + bufferSize,
				Unit:      "uS",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, reportedLatencies...)
		// crosbolt, please calculate the mins and maxes for multiple values :|
		// go/crosbolt-result-parser-g3doc#results-chartjson
		// min and max are important as latency can spike
		perfValues.Set(
			perf.Metric{
				Name:      "measured_latency_" + bufferSize + "_min",
				Unit:      "uS",
				Direction: perf.SmallerIsBetter,
			}, measuredMin)
		perfValues.Set(
			perf.Metric{
				Name:      "measured_latency_" + bufferSize + "_max",
				Unit:      "uS",
				Direction: perf.SmallerIsBetter,
			}, measuredMax)
		perfValues.Set(
			perf.Metric{
				Name:      "reported_latency_" + bufferSize + "_min",
				Unit:      "uS",
				Direction: perf.SmallerIsBetter,
			}, reportedMin)
		perfValues.Set(
			perf.Metric{
				Name:      "reported_latency_" + bufferSize + "_max",
				Unit:      "uS",
				Direction: perf.SmallerIsBetter,
			}, reportedMax)
	}
}
