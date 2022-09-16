// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mojo provides methods for running mojo service manager.
package mojo

import (
	"context"

	"chromiumos/tast/common/testexec"
)

const (
	// ActionCreateTestService is for creating test service.
	ActionCreateTestService = "create-test-service"
	// ActionPingTestService is for pinging the test service.
	ActionPingTestService = "ping-test-service"
	// ActionTestSharedBuffer is for testing the shared buffer.
	ActionTestSharedBuffer = "test-shared-buffer"
)

// CreateTestToolAction creates testexec.Cmd to run test tool with an action.
func CreateTestToolAction(ctx context.Context, action string) *testexec.Cmd {
	const testBin = "/usr/local/libexec/mojo_service_manager/test_tool"
	return testexec.CommandContext(ctx, testBin, "--action="+action)
}
