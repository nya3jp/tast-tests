// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// TODO(b/213524693) : remove "chronicler" when b/213524693 is fixed.
// TODO(b/238591902) : remove "nautilus" and "nautiluslte" when b/238591902 is fixed.
// TODO(b/238591444) : remove "soraka" when b/238591444 is fixed.
// TODO(b/238718764) : remove "karma" when b/238718764 is fixed.
// TODO(b/239385484) : remove "beetley" when b/239385484 is fixed.
// TODO(b/239385850) : remove "redrix" when b/239385850 is fixed.
// TODO(b/239409160) : remove "gimble" when b/239409160 is fixed.
// TODO(b/239412705) : remove "primus" when b/239412705 is fixed.
// TODO(b/239412769) : remove "anahera" when b/239412769 is fixed.
// TODO(b/243344261) : remove "babymega" when b/243344261 is fixed.
// TODO(b/243344614) : remove "babytiger" when b/243344614 is fixed.
// TODO(b/243345196) : remove "blacktiplte" when b/243345196 is fixed.
// TODO(b/245058202) : remove "bob" when b/245058202 is fixed.
// TODO(b/245056845) : remove "taniks" when b/245056845 is fixed.
// TODO(b/245061122) : remove "dumo" and "dru" when b/245061122 is fixed.
// TODO(b/245063090) : remove "nasher" when b/245063090 is fixed.
// TODO(b/244254621) : remove "sasukette" when b/244254621 is fixed.
// TODO(b/248997612) : remove "kevin" when b/248997612 is fixed.
// TODO(b/249023249) : remove "vell" when b/249023249 is fixed.
// TODO(b/249207920) : remove "astronaut", "blacktip", "blacktip360", "epaulette", "lava", "nasher360", "rabbid", "robo", "robo360", "santa", "whitetip" when b/249207920 is fixed.
// TODO(b/250468510) : remove "hana" when b/250468510 is fixed.
var unstableModels = []string{"chronicler", "nautilus", "nautiluslte", "soraka", "karma", "beetley", "redrix", "gimble", "primus", "anahera", "babymega", "babytiger", "blacktiplte", "taniks", "bob", "dumo", "dru", "nasher", "sasukette", "kevin", "vell", "astronaut", "blacktip", "blacktip360", "epaulette", "lava", "nasher360", "rabbid", "robo", "robo360", "santa", "whitetip", "hana"}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ALSAConformance,
		Desc:         "Runs alsa_conformance_test to test basic functions of ALSA",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Speaker(), hwdep.Microphone()),
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(unstableModels...)),
			},
			{
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(hwdep.Model(unstableModels...)),
			},
		},
	})
}

func ALSAConformance(ctx context.Context, s *testing.State) {
	// TODO(yuhsuan): Tighten the ratio if the current version is stable. (b/136614687)
	const (
		rateCriteria    = 0.1
		rateErrCriteria = 100.0
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
		s.Fatal("Failed to connect to CRAS: ", err)
	}

	// Only test on internal mic and internal speaker until below demands are met.
	// 1. Support label to force the test run on DUT having a headphone jack. (crbug.com/936807)
	// 2. Have a method to get correct PCM name from CRAS. (b/142910355).
	if err := cras.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
		s.Fatal("Failed to set internal mic active: ", err)
	}

	if err := cras.SetActiveNodeByType(ctx, "INTERNAL_SPEAKER"); err != nil {
		s.Fatal("Failed to set internal speaker active: ", err)
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
		// Sleep for 10 seconds to reset drivers and then restart CRAS.
		testing.Sleep(ctx, 10*time.Second)
		s.Log("Starting CRAS")
		if err := upstart.EnsureJobRunning(ctx, "cras"); err != nil {
			s.Fatal("Failed to start CRAS: ", err)
		}
	}(ctx)

	// Use a shorter context to save time for cleanup.
	ctx, cancel = ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// checkOutput parses and checks out, stdout from alsa_conformance_test.py.
	// It returns the number of failed tests and failure reasons.
	checkOutput := func(out []byte) (numFails int, failReasons []string) {
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
			for _, test := range suite.Tests {
				if test.Result != "pass" {
					failReasons = append(failReasons, test.Name+": "+test.Error)
				}
			}
		}

		return result.Fail, failReasons
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
			arg = "-C"
		} else {
			arg = "-P"
		}
		out, err := testexec.CommandContext(
			ctx, "alsa_conformance_test.py", arg, alsaDev,
			"--rate-criteria-diff-pct", fmt.Sprintf("%f", rateCriteria),
			"--rate-err-criteria", fmt.Sprintf("%f", rateErrCriteria),
			"--json").Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to run alsa_conformance_test: ", err)
		}

		filename := fmt.Sprintf("%s.json", stream)
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filename), out, 0644); err != nil {
			s.Error("Failed to save raw results: ", err)
		}

		fail, failReasons := checkOutput(out)

		if fail != len(failReasons) {
			s.Errorf("The number of failures and reasons does not match. (fail: %d, failReason: %d)", fail, len(failReasons))
		}

		if fail != 0 {
			s.Errorf("Device %s %s stream had %d failure(s): %q", alsaDev, stream, fail, failReasons)
		}
	}

	runTest(audio.InputStream)
	runTest(audio.OutputStream)
}
