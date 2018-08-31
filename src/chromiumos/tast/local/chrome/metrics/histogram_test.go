// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestHistogramDiff(t *testing.T) {
	parseNum := func(s string) int64 {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("Failed parsing %q: %v", s, err)
		}
		return n
	}
	makeHist := func(s string) *Histogram {
		h := &Histogram{}
		for _, b := range strings.Split(s, "|") {
			nums := strings.Fields(b)
			if len(nums) == 0 {
				continue
			} else if len(nums) != 3 {
				t.Fatalf("Didn't find \"<min> <max> <samples>\" in %q", b)
			}
			h.Buckets = append(h.Buckets, HistogramBucket{parseNum(nums[0]), parseNum(nums[1]), parseNum(nums[2])})
		}
		return h
	}

	const expectErr = "err"
	for _, tc := range []struct {
		before, after, diff string // pipe-separated "<min> <max> <count>" buckets; expectErr means error
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
		before := makeHist(tc.before)
		after := makeHist(tc.after)
		diff, err := after.Diff(before)
		if err != nil {
			if tc.diff != expectErr {
				t.Errorf("%v.Diff(%v) failed: %v", after, before, err)
			}
		} else {
			if tc.diff == expectErr {
				t.Errorf("%v.Diff(%v) = %v; want error", after, before, diff)
			} else {
				exp := makeHist(tc.diff)
				if !reflect.DeepEqual(*diff, *exp) {
					t.Errorf("%v.Diff(%v) = %v; want %v", after, before, diff, exp)
				}
			}
		}
	}
}
