// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rialto contains functionality related to Rialto devices.
//
// See http://go/rialto-spec (internal link) for more information.
package rialto

import "chromiumos/tast/lsbrelease"

// IsRialto returns true if the test appears to be running on a Rialto device.
// This should not be used to skip tests that can't be run on Rialto
// (instead, add a software dependency incorporating the "rialto" USE flag),
// but it can be used to customize a test's behavior when running on Rialto.
func IsRialto() (bool, error) {
	m, err := lsbrelease.Load()
	if err != nil {
		return false, err
	}
	return m[lsbrelease.Board] == "veyron_rialto", nil
}
