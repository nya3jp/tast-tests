// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

// Tier defines the complexity level of a CUJ test scenario.
type Tier string

// Tier enum definition.
const (
	Basic   Tier = "basic"
	Plus    Tier = "plus"
	Premium Tier = "premium"
)

// ApplicationType indicates the type of the application under test.
type ApplicationType string

// ApplicationType enum definition.
const (
	Web ApplicationType = "web"
	APP ApplicationType = "app"
)

// ScreenMode indicates the clamshell or tablet mode of the DUT.
type ScreenMode string

// ScreenMode enum definition.
const (
	Clamshell ScreenMode = "clamshell"
	Tablet    ScreenMode = "tablet"
)

// Category categorizes CUJ tests with features like tier, screenmode, and scenario.
type Category struct {
	// Tier indicates the tier the test case belongs to.
	Tier Tier
	// ScreenMode defines the clamshell or tablet mode of the DUT.
	ScreenMode ScreenMode
	// Scenario specifies the major scenario of the test. E.g. "unlock", "wakeup".
	Scenario string
}

// TestParameters is the parameters passed to each test case.
type TestParameters struct {
	// Category defines common features of the test.
	Category Category
	// Params defines test specific parameters.
	Params interface{}
}
