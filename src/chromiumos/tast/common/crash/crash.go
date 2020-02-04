// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains constants for crash reporter.
package crash

const (
	// CrashReporterPath is the full path of the crash reporter binary.
	CrashReporterPath = "/sbin/crash_reporter"

	// CrashReporterEnabledPath is the full path for crash handling data file.
	CrashReporterEnabledPath = "/var/lib/crash_reporter/crash-handling-enabled"

	// CorePattern is the full path of the core pattern file.
	CorePattern = "/proc/sys/kernel/core_pattern"
)
