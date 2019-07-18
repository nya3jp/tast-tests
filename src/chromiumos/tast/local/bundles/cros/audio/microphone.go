// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
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
		Func: Microphone,
		Desc: "Verifies microphone works correctly",
		Contacts: []string{
			"cychiang@chromium.org", // Media team
			"hidehiko@chromium.org", // Tast port author
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"audio_record"},
	})
}

func Microphone(ctx context.Context, s *testing.State) {
	const (
		duration      = 3 * time.Second
		bitReso       = 16
		tolerantRatio = 0.1
	)

	// Testing for each param.
	// - |record| is a recording function, e.g. ALSA or CRAS. (Please see
	//   recordAlsa and recordCras for the reference)
	//   |path| argument of the function is the destination path.
	//   |numChans| and |samplingRate| are as same as below.
	// - |numChans| is the number of channels for the recording.
	// - |samplingRate| is the number of samples per second.
	test := func(
		record func(path string, numChans, samplingRate int) error,
		numChans, samplingRate int) {
		tmpfile, err := ioutil.TempFile("", "audio")
		if err != nil {
			s.Fatal("Failed to create a tempfile: ", err)
		}
		defer os.Remove(tmpfile.Name())
		if err = tmpfile.Close(); err != nil {
			s.Fatal("Failed to close a tempfile: ", err)
		}

		testing.ContextLogf(ctx, "Recording... channel:%d, rate:%d", numChans, samplingRate)
		if err := record(tmpfile.Name(), numChans, samplingRate); err != nil {
			s.Error("Failed to record: ", err)
			return
		}

		info, err := os.Stat(tmpfile.Name())
		if err != nil {
			s.Error("Failed to obtain file size: ", err)
			return
		}
		expect := int(duration.Seconds()) * numChans * samplingRate * bitReso / 8
		ratio := float64(info.Size()) / float64(expect)
		if math.Abs(ratio-1.) > tolerantRatio {
			s.Errorf("File size is not correct. actual: %d, expect: %d, ratio: %f", info.Size(), expect, ratio)
			return
		}
	}

	if err := audio.WaitForDevice(ctx, audio.InputStream); err != nil {
		s.Log("Failed to wait for input stream: ", err)
		s.Log("Try to set internal mic active instead")
		if err := audio.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
			s.Fatal("Failed to set internal mic active: ", err)
		}
	}
	// Select input device.
	var inputDev string
	{
		cras, err := audio.NewCras(ctx)
		if err != nil {
			s.Fatal("Failed to connect to cras: ", err)
		}
		nodes, err := cras.GetNodes(ctx)
		if err != nil {
			s.Fatal("Failed to obtain cras nodes: ", err)
		}
		for _, n := range nodes {
			if n.Active && n.IsInput {
				inputDev = n.DeviceName
				break
			}
		}
		if inputDev == "" {
			s.Fatalf("Failed to find selected input device: %+v", nodes)
		}
		testing.ContextLogf(ctx, "Selected input device: %q", inputDev)
	}
	alsaDev := "hw:" + strings.Split(inputDev, ":")[2]

	// Look for the number of channels which ALSA supports.
	var alsaChans []int
	{
		out, err := testexec.CommandContext(ctx, "alsa_helpers", "--device", alsaDev, "--get_capture_channels").Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to get alsa recording channels: ", err)
		}
		cs := strings.Split(strings.TrimSpace(string(out)), "\n")
		alsaChans = make([]int, len(cs))
		for i, c := range cs {
			numChans, err := strconv.Atoi(c)
			if err != nil {
				s.Fatal("Failed to obtain recording channels: ", err)
			}
			alsaChans[i] = numChans
		}
	}

	// Recording function by ALSA.
	recordAlsa := func(path string, numChans int, samplingRate int) error {
		return testexec.CommandContext(
			ctx, "arecord",
			"-d", strconv.Itoa(int(duration.Seconds())),
			"-c", strconv.Itoa(numChans),
			"-f", "S16_LE",
			"-r", strconv.Itoa(samplingRate),
			"-D", "plug"+alsaDev,
			path).Run(testexec.DumpLogOnError)
	}

	// Recording function by CRAS.
	recordCras := func(path string, numChans int, samplingRate int) error {
		return testexec.CommandContext(
			ctx, "cras_test_client",
			"--capture_file", path,
			"--duration", strconv.Itoa(int(duration.Seconds())),
			"--num_channels", strconv.Itoa(numChans),
			"--rate", strconv.Itoa(samplingRate)).Run(testexec.DumpLogOnError)
	}

	// Test for each parameter.
	testing.ContextLog(ctx, "Testing ALSA")
	for _, c := range alsaChans {
		test(recordAlsa, c, 44100)
		test(recordAlsa, c, 48000)
	}
	testing.ContextLog(ctx, "Testing Cras")
	test(recordCras, 1, 44100)
	test(recordCras, 1, 48000)
	test(recordCras, 2, 44100)
	test(recordCras, 2, 48000)
}
