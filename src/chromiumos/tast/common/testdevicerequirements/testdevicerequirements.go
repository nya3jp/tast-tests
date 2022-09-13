// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testdevicerequirements provides a way to define ChromeOS
// software and device requirements used for mapping test cases to
// requirements
//
// # Usage
//
// import tdreq "chromiumos/tast/common/testdevicerequirement"
//
// testing.AddTest(&testing.Test{
//
//	Func:         ExampleTest,
//	LacrosStatus: testing.LacrosVariantNeeded,
//	Desc:         "Fake example test",
//	...
//	Requirements: []string{tdreq.BootPerfKernel, tdreq.BootPerfLogin},
//	...
//
// )
//
// Contact cros-test-xfn-requirements@google.com with any questions.
package testdevicerequirements

const (
	// BootPerfKernel is a ChromeOS Platform requirement. Please contact cros-test-xfn-requirements@google.com with any questions.
	BootPerfKernel = "boot-perf-0001-v01"

	// BootPerfLogin is a ChromeOS Platform requirement. Please contact cros-test-xfn-requirements@google.com with any questions.
	BootPerfLogin = "boot-perf-0002-v01"
)
