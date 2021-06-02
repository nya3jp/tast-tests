// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/memory/stress"
	"chromiumos/tast/testing"
)

type testCaseResult = stress.TestCaseResult

const (
	allocPageFilename  = "memory_stress.html"
	javascriptFilename = "memory_stress.js"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryStress,
		Desc:     "Tests lacros under heavy memory pressure",
		Contacts: []string{"vovoy@chromium.org", "chromeos-memory@google.com"},
		Timeout:  15 * time.Minute,
		Data: []string{
			allocPageFilename,
			javascriptFilename,
			launcher.DataArtifact,
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "lacrosStartedByData",
		Vars:         []string{"lacros.MemoryStress.seed"},
	})

}

func MemoryStress(ctx context.Context, s *testing.State) {
	const (
		mbPerTab    = 800
		switchCount = 10
	)

	seed := time.Now().UTC().UnixNano()
	if val, ok := s.Var("lacros.MemoryStress.seed"); ok {
		intVal, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			s.Fatal("Cannot parse argument lacros.MemoryStress.seed: ", err)
		}
		seed = intVal
	}
	testing.ContextLog(ctx, "Seed: ", seed)
	localRand := rand.New(rand.NewSource(seed))

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL := server.URL + "/" + allocPageFilename

	perfValues := perf.NewValues()

	result, err := stressTestCase(ctx, s, localRand, mbPerTab, switchCount, baseURL)
	if err != nil {
		s.Fatal("Stress test case failed: ", err)
	}

	if err := stress.ReportTestCaseResult(ctx, perfValues, result, "stress"); err != nil {
		s.Fatal("Reporting result failed: ", err)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

func stressTestCase(ctx context.Context, s *testing.State, localRand *rand.Rand, mbPerTab, switchCount int, baseURL string) (testCaseResult, error) {
	lacros, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData), s.DataPath(launcher.DataArtifact))
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to launch lacros-chrome")
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to wait for idle CPU")
	}

	const compressRatio = 0.67
	return stress.TestCase(ctx, lacros, localRand, mbPerTab, switchCount, compressRatio, baseURL)
}
