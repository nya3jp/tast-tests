// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// makeHist parses a sequence of pipe-separated "<min> <max> <count>" buckets,
// e.g. "0 5 1 | 5 10 3 | 10 15 2" for [0,5):1 [5,10):3 [10,15):2.
func makeHist(t *testing.T, s string) *Histogram {
	t.Helper()

	parseNum := func(s string) int64 {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("Failed parsing %q: %v", s, err)
		}
		return n
	}

	h := &Histogram{}
	for _, b := range strings.Split(s, "|") {
		nums := strings.Fields(b)
		if len(nums) == 0 {
			continue
		} else if len(nums) != 3 {
			t.Fatalf("Didn't find \"<min> <max> <samples>\" in %q", b)
		}
		h.Buckets = append(h.Buckets, HistogramBucket{parseNum(nums[0]), parseNum(nums[1]), parseNum(nums[2])})
		// Compute the sum from the bucket for testing. For now, it estimates all
		// samples are in the minimum value.
		h.Sum += parseNum(nums[0]) * parseNum(nums[2])
	}
	return h
}

func TestHistogramValidate(t *testing.T) {
	for _, tc := range []struct {
		buckets string // passed to makeHist
		valid   bool
	}{
		{"", true},
		{"0 3 4", true},
		{"6 8 1", true},
		{"-1 1 1", true},
		{"0 3 4 | 3 6 8", true},
		{"0 1 4 | 1 2 8 | 2 3 5", true},
		{"2 2 5", false},          // max must be >= min
		{"3 2 5", false},          // max must be >= min
		{"5 6 1 | 4 5 2", false},  // buckets must be increasing
		{"0 5 1 | 4 10 2", false}, // buckets can't overlap
	} {
		h := makeHist(t, tc.buckets)
		err := h.validate()
		if err != nil && tc.valid {
			t.Errorf("validate() failed for %v: %v", *h, err)
		} else if err == nil && !tc.valid {
			t.Errorf("validate() unexpectedly succeeded for %v", *h)
		}
	}
}

func TestHistogramDiff(t *testing.T) {
	const expectErr = "err"
	for _, tc := range []struct {
		before, after, diff string // passed to makeHist
	}{
		{
			before: "0 5 1 | 5 10 2 | 10 20 2",
			after:  "0 5 3 | 5 10 6 | 10 20 3",
			diff:   "0 5 2 | 5 10 4 | 10 20 1",
		},
		{
			before: "0 5 1 | 5 10 2 |        ",
			after:  "0 5 3 | 5 10 6 | 10 20 3",
			diff:   "0 5 2 | 5 10 4 | 10 20 3",
		},
		{
			before: "0 5 1 |        | 10 20 2",
			after:  "0 5 3 | 5 10 6 | 10 20 3",
			diff:   "0 5 2 | 5 10 6 | 10 20 1",
		},
		{
			before: "      | 5 10 1 | 10 20 2",
			after:  "0 5 3 | 5 10 6 | 10 20 3",
			diff:   "0 5 3 | 5 10 5 | 10 20 1",
		},
		{
			before: "      |        |        ",
			after:  "0 5 3 | 5 10 6 | 10 20 3",
			diff:   "0 5 3 | 5 10 6 | 10 20 3",
		},
		{
			before: "      | 5 10 2 |        ",
			after:  "0 5 3 | 5 10 6 | 10 20 3",
			diff:   "0 5 3 | 5 10 4 | 10 20 3",
		},
		{
			before: "0 5 1 | 5 10 2 | 10 20 2",
			after:  "0 5 1 | 5 10 2 | 10 20 2",
			diff:   "", // unchanged (same values)
		},
		{
			before: "",
			after:  "",
			diff:   "", // unchanged (both empty)
		},
		{
			before: "0 5 2",
			after:  "0 5 1",
			diff:   expectErr, // count decreased
		},
		{
			before: "0 4 1",
			after:  "0 5 2",
			diff:   expectErr, // same min but different max
		},
		{
			before: "0 5 1",
			after:  "1 5 2",
			diff:   expectErr, // same max but before min is less
		},
		{
			before: "1 5 1",
			after:  "0 5 2",
			diff:   expectErr, // same max but before min is greater
		},
		{
			before: "0 5 1 | 5 10 2 | 10 20 3",
			after:  "0 5 2 |        | 10 20 4",
			diff:   expectErr, // before has bucket not in after
		},
		{
			before: "1 5 1",
			after:  "",
			diff:   expectErr, // after is empty
		},
	} {
		before := makeHist(t, tc.before)
		after := makeHist(t, tc.after)
		diff, err := after.Diff(before)
		if err != nil {
			if tc.diff != expectErr {
				t.Errorf("%v.Diff(%v) failed: %v", after, before, err)
			}
		} else {
			if tc.diff == expectErr {
				t.Errorf("%v.Diff(%v) = %v; want error", after, before, diff)
			} else {
				exp := makeHist(t, tc.diff)
				if !reflect.DeepEqual(*diff, *exp) {
					t.Errorf("%v.Diff(%v) = %v; want %v", after, before, diff, exp)
				}
			}
		}
	}
}

func TestClearHistogramTransferFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestClearHistogramTransferFile")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	fileName := dir + "/metrics"
	const contents = "ABC123"
	if err = ioutil.WriteFile(fileName, []byte(contents), 0666); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if err = clearHistogramTransferFileByName(fileName); err != nil {
		t.Fatalf("clearHistogramTransferFileByName: %v", err)
	}

	if info, err := os.Stat(fileName); err != nil {
		t.Fatalf("os.Stat: %v", err)
	} else if info.Size() != 0 {
		t.Error("file was not truncated")
	}
}

func TestClearHistogramTransferFileWhenFileDoesntExist(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestClearHistogramTransferFileWhenFileDoesntExist")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	fileName := dir + "/metrics"
	// Don't create the file.
	if err = clearHistogramTransferFileByName(fileName); err != nil {
		t.Fatalf("clearHistogramTransferFileByName: %v", err)
	}

	_, err = os.Stat(fileName)

	if err == nil {
		t.Error("file was created")
	} else if !os.IsNotExist(err) {
		t.Errorf("os.Stat: %v", err)
	}
}
