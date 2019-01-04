// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"encoding/json"
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
	CheckOutput := func(ctx context.Context, s *testing.State, out []byte) (int, []string) {
		var result map[string]interface{}
		json.Unmarshal(out, &result)

		pass := int(result["PASS"].(float64))
		fail := int(result["FAIL"].(float64))

		testing.ContextLogf(ctx, "Test Result: %d PASS %d FAIL", pass, fail)
		testSuite := []string{"Test Params"}

		// Traverse all test results to get failure suites.
		var failSuite []string
		for _, testName := range testSuite {
			testing.ContextLogf(ctx, "%s", testName)
			testResult := result[testName].(map[string]interface{})
			testData := testResult["data"].([]interface{})
			if int(testResult["FAIL"].(float64)) != 0 {
				failSuite = append(failSuite, testName)
			}
			for _, value := range testData {
				data := value.(map[string]interface{})
				msg := data["test"].(string) + ": " + data["result"].(string)
				if data["result"].(string) == "FAIL" {
					msg += "- " + data["error"].(string)
				}
				testing.ContextLogf(ctx, "\t%s", msg)
			}
		}

		return fail, failSuite
	}

	RunTest := func(ctx context.Context, s *testing.State, stream string) {
		var node audio.CrasNode
		cras, err := audio.NewCras(ctx)
		if err != nil {
			s.Fatal("Failed to connect to cras: ", err)
		}

		if stream == "input" {
			node, err = cras.GetSelectedInputNode(ctx)
		} else {
			node, err = cras.GetSelectedOutputNode(ctx)
		}
		if err != nil {
			s.Fatal("Failed to find selected device: ", err)
		}

		testing.ContextLogf(ctx, "Selected %s device: %s", stream, node.DeviceName)
		alsaDev := "hw:" + strings.Split(node.DeviceName, ":")[2]
		testing.ContextLogf(ctx, "Run alsa_conformance_test on %s device %s", stream, alsaDev)

		var arg string
		if stream == "input" {
			arg = "CAPTURE"
		} else {
			arg = "PLAYBACK"
		}
		cmd := testexec.CommandContext(ctx, "alsa_conformance_test.py", alsaDev, arg, "--json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			cmd.DumpLog(ctx)
			testing.ContextLogf(ctx, "%s", out)
			s.Fatal("Failed: ", err)
		}

		fail, failSuite := CheckOutput(ctx, s, out)

		if fail != 0 {
			s.Errorf("Input device %s failed %d times. Suite: %s", alsaDev, fail, failSuite)
		}
	}

	RunTest(ctx, s, "input")
	RunTest(ctx, s, "output")
}
