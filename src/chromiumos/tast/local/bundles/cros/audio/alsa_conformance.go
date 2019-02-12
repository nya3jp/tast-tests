// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ALSAConformance,
		Desc:         "Runs alsa_conformance_test to test basic functions of ALSA",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"audio_play", "audio_record"},
	})
}

func ALSAConformance(ctx context.Context, s *testing.State) {
	if err := audio.WaitForDevice(ctx, audio.InputStream|audio.OutputStream); err != nil {
		s.Fatal("Failed to wait for input and output streams: ", err)
	}

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to CRAS: ", err)
	}

	crasNodes, err := cras.GetNodes(ctx)
	if err != nil {
		s.Fatal("Failed to obtain CRAS nodes: ", err)
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
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// checkOutput parses and checks out, stdout from alsa_conformance_test.py.
	// It returns the number of failed tests and failed test suites.
	checkOutput := func(out []byte) (numFails int, failSuites []string) {
		result := struct {
			Pass       int `json:"pass"`
			Fail       int `json:"fail"`
			TestSuites []struct {
				Name  string `json:"name"`
				Pass  int    `json:"pass"`
				Fail  int    `json:"fail"`
				Tests []struct {
					Name   string `json:"name"`
					Result string `json:"result"`
					Error  string `json:"error"`
				} `json:"tests"`
			} `json:"testSuites"`
		}{}

		if err := json.Unmarshal(out, &result); err != nil {
			s.Fatal("Failed to unmarshal test results: ", err)
		}
		s.Logf("alsa_conformance_test.py results: %d passed %d failed", result.Pass, result.Fail)

		for _, suite := range result.TestSuites {
			if suite.Fail != 0 {
				failSuites = append(failSuites, suite.Name)
			}
		}

		return result.Fail, failSuites
	}

	runTest := func(stream audio.StreamType) {

		var node *audio.CrasNode
		for i, n := range crasNodes {
			if n.Active && n.IsInput == (stream == audio.InputStream) {
				node = &crasNodes[i]
				break
			}
		}

		if node == nil {
			s.Fatal("Failed to find selected device: ", err)
		}

		s.Logf("Selected %s device: %s", stream, node.DeviceName)
		alsaDev := "hw:" + strings.Split(node.DeviceName, ":")[2]
		s.Logf("Running alsa_conformance_test on %s device %s", stream, alsaDev)

		var arg string
		if stream == audio.InputStream {
			arg = "CAPTURE"
		} else {
			arg = "PLAYBACK"
		}
		cmd := testexec.CommandContext(ctx, "alsa_conformance_test.py", alsaDev, arg, "--json")
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Fatal("Failed: ", err)
		}

		filename := string(stream) + ".json"
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filename), out, 0644); err != nil {
			s.Error("Failed to save raw results: ", err)
		}

		fail, failSuites := checkOutput(out)

		if fail != 0 {
			s.Errorf("Device %s %s stream had %d failure(s): %s", alsaDev, stream, fail, failSuites)
		}
	}

	runTest(audio.InputStream)
	runTest(audio.OutputStream)
}
