// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

// Tier defines the complexity level of a CUJ test scenario.
type Tier int

const (
	// TODO: When basic, plus, premium are no longer used, remove them.

	// Basic is the test tier covering some simple CUJ test scenarios. Basic tier CUJ tests are supposed
	// to run on most of the DUT models (both high-end and low-end), if not all.
	Basic Tier = iota
	// Plus is the test tier covering more test scenarios than basic tier.
	Plus
	// Premium is the test tier covering the most complex test scenarios. CUJ tests in premium tier will
	// drive the DUT to use more of its system resources.
	Premium

	// Essential and Advanced are the tiers used for Spera V2.

	// Essential is the test tier covering some simple CUJ test scenarios. Essential tier CUJ tests are supposed
	// to run on most of the DUT models (both high-end and low-end), if not all.
	Essential
	// Advanced is the test tier covering the most complex test scenarios. CUJ tests in advanced tier will
	// drive the DUT to use more of its system resources.
	Advanced
)
