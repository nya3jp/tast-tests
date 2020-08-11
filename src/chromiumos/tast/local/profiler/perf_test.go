// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/testutil"
)

func TestHandleStatOnly(t *testing.T) {
	// The test data comes from command like
	// perf stat -a -p 8113 -e cycles --output perf_stat_only.data
	const data = `# started on Tue Aug 11 17:50:16 2020


	 Performance counter stats for process id '8113':

	          190435360      cycles

		         7.999881391 seconds time elapsed

	`

	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "perf_stat_only.data")
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal("Failed to create perf_stat_only.data: ", err)
	}

	cyclesPerSecond, err := parseStatFile(path)
	if err != nil {
		t.Fatal("Failed to parse stat file: ", err)
	}

	expected := 23804772.932539094

	if !cmp.Equal(cyclesPerSecond, expected, cmp.Comparer(func(x, y float64) bool {
		return math.Abs(x-y) < 1e-6
	})) {
		t.Errorf("Result not match, got %#v want %#v", cyclesPerSecond, expected)
	}
}
