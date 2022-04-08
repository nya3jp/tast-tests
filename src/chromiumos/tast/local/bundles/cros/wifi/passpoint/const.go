// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

const (
	// ProfileUser is the name of the fake test user profile
	ProfileUser = "test"
)

// Domains and Organisation Identifier (OI) used to test Passpoint
// network selection. The domains and OIs are extracted from Passpoint
// specification v3.2 - Appendix C.
const (
	BlueDomain         = "sp-blue.com"
	GreenDomain        = "sp-green.com"
	RedDomain          = "sp-red.com"
	HomeOI      uint64 = 0x871d2e
	RoamingOI1  uint64 = 0x1bc50050
	RoamingOI2  uint64 = 0x1bc500b5
)
