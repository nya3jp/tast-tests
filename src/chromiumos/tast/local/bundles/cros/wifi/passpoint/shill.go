// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
	"math/rand"
)

// RandomProfileName returns a random name for Shill test profile.
func RandomProfileName() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	s := make([]byte, 8)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return "passpoint" + string(s)
}
