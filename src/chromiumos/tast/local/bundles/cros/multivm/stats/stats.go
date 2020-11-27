// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stats

import (
	"math"
	"math/rand"
	"strconv"
)

// ExponentialInt64 generates a random int64 from an exponential distribution
// with the passed mean. If r is nil, returns mean.
func ExponentialInt64(mean int64, r *rand.Rand) int64 {
	if r == nil {
		return mean
	}
	return int64(-math.Log(1-r.Float64()) * float64(mean))
}

// NewRandFromVar creats a new rand.Rand from the result of a testing.State.Var
// containing a boolean. If the Var does not exist or is false, then nil is
// returned.
func NewRandFromVar(varStr string, ok bool) *rand.Rand {
	if !ok {
		return nil
	}
	if b, err := strconv.ParseBool(varStr); err != nil || !b {
		return nil
	}

	// Chosen by fair dice roll in https://xkcd.com/221. We want consistent
	// results from test to test, so don't change it.
	return rand.New(rand.NewSource(4))
}
