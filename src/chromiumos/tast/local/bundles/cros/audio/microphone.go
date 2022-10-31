// Copyright 2018 The ChromiumOS Authors
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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Microphone,
		Desc: "Verifies microphone works correctly",
		Contacts: []string{
			"cychiang@chromium.org", // Media team
			"hidehiko@chromium.org", // Tast port author
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"audio_stable"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
	})
}

func Microphone(ctx context.Context, s *testing.State) {
	const (
		duration      = 3 * time.Second
		bitReso       = 16
		tolerantRatio = 0.1
	)

	// Stop UI in advance for this test to avoid the node being selected by UI.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Use a shorter context to save time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to cras: ", err)
	}

	// There is no way to query which device is used by CRAS now. However, the
	// PCM name of internal mic is still correct, we can always run test on the
	// internal mic until there is a method to get the correct device name.
	// See b/142910355 for more details.
	if err := cras.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
		s.Fatal("Failed to set internal mic active: ", err)
	}

	// Select input device.
	var inputDev string
	{
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
	// - path argument of the function is the destination path.
	// - numChans and samplingRate are as same as below.
	recordAlsa := func(path string, numChans, samplingRate int) error {
		return testexec.CommandContext(
			ctx, "arecord",
			"-d", strconv.Itoa(int(duration.Seconds())),
			"-c", strconv.Itoa(numChans),
			"-f", "S16_LE",
			"-r", strconv.Itoa(samplingRate),
			"-D", "plug"+alsaDev,
			path).Run(testexec.DumpLogOnError)
	}

	// Stop CRAS to make sure the audio device won't be occupied.
	s.Log("Stopping CRAS")
	if err := upstart.StopJob(ctx, "cras"); err != nil {
		s.Fatal("Failed to stop CRAS: ", err)
	}

	defer func(ctx context.Context) {
		// Restart CRAS.
		s.Log("Starting CRAS")
		if err := upstart.EnsureJobRunning(ctx, "cras"); err != nil {
			s.Fatal("Failed to start CRAS: ", err)
		}
	}(ctx)

	// Use a shorter context to save time for cleanup.
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Testing for each param.
	// - numChans is the number of channels for the recording.
	// - samplingRate is the number of samples per second.
	test := func(numChans, samplingRate int) {
		tmpfile, err := ioutil.TempFile("", "audio")
		if err != nil {
			s.Fatal("Failed to create a tempfile: ", err)
		}
		defer os.Remove(tmpfile.Name())
		if err = tmpfile.Close(); err != nil {
			s.Fatal("Failed to close a tempfile: ", err)
		}

		testing.ContextLogf(ctx, "Recording... channel:%d, rate:%d", numChans, samplingRate)
		if err := recordAlsa(tmpfile.Name(), numChans, samplingRate); err != nil {
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

	// Test for each parameter.
	testing.ContextLog(ctx, "Testing ALSA")
	for _, c := range alsaChans {
		test(c, 44100)
		test(c, 48000)
	}
}
