// Copyright 2022 The ChromiumOS Authors.
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
//	Requirements: []string{tdreq.BootPerformance1, tdreq.BootPerformance2},
//	...
//
// )
package testdevicerequirements

const (
	// BootPerformance1 specifies The Chrome device MUST take no longer than 1 second to boot
	// from Deep Sleep (ACPI S5) to kernel execution.
	BootPerformance1 = "boot-perf-0001-v01"

	// BootPerformance2 specifies The Chrome device MUST take no longer than 8 seconds total to
	// boot from Deep Sleep (ACPI S5) to the login screen in the normal
	// boot process, or the firmware recovery screen in the recovery boot
	// process.
	BootPerformance2 = "boot-perf-0002-v01"
)
