// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hpsutil

import (
	"math"
	"testing"
)

func TestPercentile(t *testing.T) {
	for _, tc := range []struct {
		data []float64
		p    int
		res  float64
		err  string
	}{
		{
			data: []float64{},
			p:    0,
			res:  math.NaN(),
			err:  "empty input",
		},
		{
			data: []float64{10},
			p:    0,
			res:  10,
			err:  "",
		},
		{
			data: []float64{10},
			p:    99,
			res:  10,
			err:  "",
		},
		{
			data: []float64{10},
			p:    -1,
			res:  math.NaN(),
			err:  "invalid percentile bounds: -1",
		},
		{
			data: []float64{10},
			p:    100,
			res:  math.NaN(),
			err:  "invalid percentile bounds: 100",
		},
		{
			data: []float64{10, 20, 30},
			p:    0,
			res:  10,
			err:  "",
		},
		{
			data: []float64{10, 20, 30},
			p:    50,
			res:  20,
			err:  "",
		},
		{
			data: []float64{10, 20, 30},
			p:    99,
			res:  30,
			err:  "",
		},
	} {
		result, err := PercentileForSortedData(tc.data, tc.p)
		if tc.err == "" {
			if err != nil {
				t.Errorf("1: (%v, %d): Unexpectedly failed (%f, %v)", tc.data, tc.p, result, err)
			} else if math.IsNaN(result) && math.IsNaN(tc.res) {
			} else if result != tc.res {
				t.Errorf("2: (%v, %d): Expected (%f) Got (%f)", tc.data, tc.p, tc.res, result)
			}
		} else {
			if err == nil {
				t.Errorf("3: (%v, %d): Expected (%f, %v) Got (%f, %v)", tc.data, tc.p, tc.res, tc.err, result, err)
			} else if err.Error() != tc.err {
				t.Errorf("4: (%v, %d): Expected (%v) Got (%v)", tc.data, tc.p, tc.err, err)
			} else if math.IsNaN(result) && math.IsNaN(tc.res) {
			} else if result != tc.res {
				t.Errorf("5: (%v, %d): Expected (%f) Got (%f)", tc.data, tc.p, tc.res, result)
			}
		}
	}
}
