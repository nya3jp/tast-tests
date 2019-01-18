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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ALSAConformance,
		Desc:         "Runs alsa_conformance_test to test basic functions of ALSA",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"audio_play", "audio_record"},
	})
}

func ALSAConformance(ctx context.Context, s *testing.State) {
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

	const cleanupTime = 5 * time.Second
	defer func(ctx context.Context) {
		// Restart CRAS.
		s.Log("Starting CRAS")
		if err := upstart.EnsureJobRunning(ctx, "cras"); err != nil {
			s.Fatal("Failed to start CRAS: ", err)
		}
		// Wait until UI selects active nodes.
		checkActiveNodes := func(ctx context.Context) error {
			cras, err := audio.NewCras(ctx)
			if err != nil {
				return err
			}
			crasNodes, err := cras.GetNodes(ctx)
			if err != nil {
				return err
			}
			for _, n := range crasNodes {
				if n.Active {
					return nil
				}
			}
			return errors.New("no active nodes")
		}
		err := testing.Poll(ctx, checkActiveNodes, &testing.PollOptions{Timeout: cleanupTime})
		if err != nil {
			s.Fatal("Failed to check active nodes: ", err)
		}
	}(ctx)

	// Use a shorter context to save time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
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

	type streamType string
	const (
		inputStream  streamType = "input"
		outputStream            = "output"
	)

	runTest := func(stream streamType) {

		var node *audio.CrasNode
		for i, n := range crasNodes {
			if n.Active && n.IsInput == (stream == inputStream) {
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
		if stream == inputStream {
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

	runTest(inputStream)
	runTest(outputStream)
}
