// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package aggregate contains aggregation funcitons used to combine different
// sub test results.
package aggregate

import "math"

// Max returns the maximum value.
func Max(xs ...float64) float64 {
	var result = -math.MaxFloat64
	for _, x := range xs {
		if result < x {
			result = x
		}
	}
	return result
}

// Mean returns arithmetic mean.
func Mean(xs ...float64) float64 {
	if len(xs) < 1 {
		return 0.0
	}
	// Formula for numerically stable mean computation from The Art Of Computer
	// Programming Volume 2, Section 4.2.2, Equation 15
	avg := xs[0]
	for i := 1; i < len(xs); i++ {
		avg = avg + (xs[i]-avg)/float64(i+1)
	}
	return avg
}
