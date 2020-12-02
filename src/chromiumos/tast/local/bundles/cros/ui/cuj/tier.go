// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

// Tier defines the complexity level of a CUJ test scenario.
type Tier int

const (
	// Basic is the test tier covering basic CUJ test scenarios.
	Basic Tier = iota
	// Plus is the test tier covering more test scenarios than basic tier.
	Plus
	// Premium is the test tier covering the most complex test scenarios.
	Premium
)
