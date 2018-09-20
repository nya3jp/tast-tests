// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Microphone,
		Desc:         "Verify microphone works correctly.",
		SoftwareDeps: []string{"audio_record"},
	})
}

func Microphone(s *testing.State) {
	const (
		duration      = 3 * time.Second
		bitReso       = 16
		tolerantRatio = 0.1
	)

	ctx := s.Context()

	test := func(
		record func(path string, numChans int, samplingRate int) error,
		numChans int, samplingRate int) {
		path, err := ioutil.TempFile("", "audio")
		if err != nil {
			s.Fatal("Failed to create a tempfile: ", err)
		}
		testing.ContextLogf(ctx, "Recording... channel:%d, rate:%d", numChans, samplingRate)
		if err := record(path.Name(), numChans, samplingRate); err != nil {
			s.Error("Failed to record by ALSA: ", err)
			return
		}

		info, err := os.Stat(path.Name())
		if err != nil {
			s.Error("Failed to obtain file size: ", err)
			return
		}
		expect := int(duration.Seconds()) * numChans * samplingRate * bitReso / 8
		if math.Abs(float64(info.Size())/float64(expect)-1.) > tolerantRatio {
			s.Error("File size is not correct. actual: %d, expect: %d", info.Size(), expect)
			return
		}
	}

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to cras: ", err)
	}
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		s.Fatal("Failed to obtain cras nodes: ", err)
	}
	inputDev := ""
	for _, n := range nodes {
		if n.Active && n.IsInput {
			inputDev = n.DeviceName
			break
		}
	}
	if inputDev == "" {
		s.Fatal("Failed to find selected input device: %v", nodes)
	}
	testing.ContextLogf(ctx, "Selected input device: %q", inputDev)
	alsaDev := "hw:" + strings.Split(inputDev, ":")[2]

	var alsaChans []int
	{
		cmd := testexec.CommandContext(ctx, "alsa_helpers", "--device", alsaDev, "--get_capture_channels")
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Fatal("Failed to get alsa recording channels: ", err)
		}
		cs := strings.Split(strings.TrimSpace(string(out)), "\n")
		alsaChans = make([]int, len(cs))
		for i, c := range cs {
			numChans, err := strconv.Atoi(c)
			if err != nil {
				cmd.DumpLog(ctx)
				s.Fatal("Failed to obtain recording channels: ", err)
			}
			alsaChans[i] = numChans
		}
	}

	recordAlsa := func(path string, numChans int, samplingRate int) error {
		cmd := testexec.CommandContext(
			ctx, "arecord",
			"-d", strconv.Itoa(int(duration.Seconds())),
			"-c", strconv.Itoa(numChans),
			"-f", "S16_LE",
			"-r", strconv.Itoa(samplingRate),
			"-D", "plug"+alsaDev,
			path)
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return fmt.Errorf("Failed to record: ", err)
		}
		return nil
	}
	recordCras := func(path string, numChans int, samplingRate int) error {
		cmd := testexec.CommandContext(
			ctx, "cras_test_client",
			"--capture_file", path,
			"--duration", strconv.Itoa(int(duration.Seconds())),
			"--num_channels", strconv.Itoa(numChans),
			"--rate", strconv.Itoa(samplingRate))
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return fmt.Errorf("Failed to record: ", err)
		}
		return nil
	}

	testing.ContextLogf(ctx, "Testing ALSA")
	for _, c := range alsaChans {
		test(recordAlsa, c, 44100)
		test(recordAlsa, c, 48000)
	}

	testing.ContextLogf(ctx, "Testing Cras")
	test(recordCras, 1, 44100)
	test(recordCras, 1, 48000)
	test(recordCras, 2, 44100)
	test(recordCras, 2, 48000)
}
