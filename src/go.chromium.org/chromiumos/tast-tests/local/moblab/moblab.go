// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package moblab contains functionality related to the Moblab automated testing environment.
//
// See https://www.chromium.org/chromium-os/testing/moblab for more information.
package moblab

import "os/user"

// IsMoblab returns true if the test appears to be running on a Moblab device.
// Moblab devices frequently have additional services installed and running, so
// security tests may use this function to determine if additional information
// needs to be added to baselines.
func IsMoblab() bool {
	_, err := user.Lookup("moblab")
	return err == nil
}
