// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chromiumos/tast/testutil"
)

func TestParseStatFile(t *testing.T) {
	// The test data comes from command like
	// perf stat -a -p 8113 -e cycles --output perf_stat_only.data
	const data = `# started on Tue Aug 11 17:50:16 2020


	 Performance counter stats for process id '8113':

	          190435360      cycles

		         7.999881391 seconds time elapsed

	`

	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "perf_stat.data")
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal("Failed to create perf_stat.data: ", err)
	}

	cyclesPerSecond, err := parseStatFile(path)
	if err != nil {
		t.Fatal("Failed to parse stat file: ", err)
	}

	expected := 23804772.932539094

	if math.Abs(expected-cyclesPerSecond) >= 1e-6 {
		t.Errorf("Unexpected cycles per second: got %#v; want %#v", cyclesPerSecond, expected)
	}
}

func TestParseStatFileNoCycle(t *testing.T) {
	// The test data comes from command like
	// perf stat -a -p 8113 -e cycles --output perf_stat_only.data
	const data = `# started on Tue Aug 11 17:50:16 2020


	 Performance counter stats for process id '8113':

	          <not counted>      cycles

		         7.999881391 seconds time elapsed

	`

	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "perf_stat_no_cycle.data")
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal("Failed to create perf_stat_no_cycle.data: ", err)
	}

	_, err := parseStatFile(path)
	if err == nil || !strings.Contains(err.Error(), "got 0 cycle") {
		t.Fatal("Failed to check stat file with no cycle: ", err)
	}
}
