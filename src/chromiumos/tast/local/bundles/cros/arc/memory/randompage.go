// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file initializes 4096 random bytes, to be used to fill pages so that
// page compression does make any allocations smaller.
// N.B. page compression just does page at a time, so we can re-use.

package memory

import (
	"math/rand"
)

var randomPage [4096]byte

func init() {
	// Make sure random numbers are consistently seeded, 1 is default.
	rand.New(rand.NewSource(1)).Read(randomPage[:])
}
