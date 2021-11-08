// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"math"
	"math/rand"
	"strconv"
	"time"
)

// RandomDeviceName appends a randomly generated integer (up to 6 digits) to the base device name to avoid conflicts
// when nearby devices in the lab may be running the same test at the same time.
func RandomDeviceName(basename string) string {
	const maxDigits = 6
	rand.Seed(time.Now().UnixNano())
	num := rand.Intn(int(math.Pow10(maxDigits) - 1))
	return basename + strconv.Itoa(num)
}
