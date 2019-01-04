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

	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AlsaConformance,
		Desc:         "Run alsa_conformace_test.py to test basic functions of ALSA.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"audio_play", "audio_record"},
	})
}

func AlsaConformance(ctx context.Context, s *testing.State) {

	// checkOutput parses and checks the output from alsa_conformance_test.py
	// |out| the json output from alsa_conformance_test.py. For example,
	// {
	//   "FAIL":1,
	//   "PASS":4,
	//   "Test Suites":[
	//      {
	//         "name":"Test Params",
	//         "FAIL":1,
	//         "PASS":4,
	//         "data":[
	//            {
	//               "test":"Set channels 2",
	//               "result":"PASS",
	//               "error":""
	//            },
	//            {
	//               "test":"Set rate 48000",
	//               "result":"FAIL",
	//               "error":"Set rate 48000 but got 44100"
	//            }
	//         ],
	//      }
	//   ]
	// }
	// |numFails| the number represents how many tests failed.
	// |failSuites| the string array contains failed test suites.
	checkOutput := func(out []byte) (numFails int, failSuites []string) {
		type Test struct {
			Test   string `json"test"`
			Result string `json"result"`
			Error  string `json"error"`
		}

		type TestSuite struct {
			Name  string `json"name"`
			Pass  int    `json:"PASS"`
			Fail  int    `json:"FAIL"`
			Tests []Test `json:"data"`
		}

		result := struct {
			Pass       int         `json:"PASS"`
			Fail       int         `json:"FAIL"`
			TestSuites []TestSuite `json:"Test Suites"`
		}{}

		if err := json.Unmarshal(out, &result); err != nil {
			s.Fatalf("Unmarshal failed: %s, out = %s", err, string(out))
		}
		s.Logf("Test Result: %d PASS %d FAIL", result.Pass, result.Fail)

		// Save the parsed result.
		log, err := json.MarshalIndent(result, "", "\t")
		if err != nil {
			s.Error("MarshalIndent:", err)
		}
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "result.json"), log, 0644); err != nil {
			s.Error(err)
		}

		for _, suite := range result.TestSuites {
			if suite.Fail != 0 {
				failSuites = append(failSuites, suite.Name)
			}
		}

		return result.Fail, failSuites
	}

	runTest := func(stream string) {
		cras, err := audio.NewCras(ctx)
		if err != nil {
			s.Fatal("Failed to connect to cras: ", err)
		}

		nodes, err := cras.GetNodes(ctx)
		if err != nil {
			s.Fatal("Failed to obtain cras nodes: ", err)
		}

		var inputFlag bool
		if stream == "input" {
			inputFlag = true
		} else {
			inputFlag = false
		}

		var node *audio.CrasNode
		for i, n := range nodes {
			if n.Active && n.IsInput == inputFlag {
				node = &nodes[i]
				break
			}
		}

		if node == nil {
			s.Fatal("Failed to find selected device: ", err)
		}

		s.Logf("Selected %s device: %s", stream, node.DeviceName)
		alsaDev := "hw:" + strings.Split(node.DeviceName, ":")[2]
		s.Logf("Run alsa_conformance_test on %s device %s", stream, alsaDev)

		var arg string
		if stream == "input" {
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

		fail, failSuites := checkOutput(out)

		if fail != 0 {
			s.Errorf("The %s device %s failed %d times. Suite: %s", stream, alsaDev, fail, failSuites)
		}
	}

	runTest("input")
	runTest("output")
}
