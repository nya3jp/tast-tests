// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mojo tests the system daemon mojo_service_manager to verify the mojo
// functionality.
package mojo

import (
	"context"
	"time"

	mojoutil "chromiumos/tast/local/mojo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServiceManagerTestService,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check we can register and request service from mojo service manager",
		Contacts: []string{
			"chromeos-mojo-service-manager@google.com",
			"chungsheng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"mojo_service_manager"},
		Timeout:      1 * time.Minute,
		Fixture:      "mojoServiceManagerRunning",
	})
}

func ServiceManagerTestService(ctx context.Context, s *testing.State) {
	serviceProc := mojoutil.CreateTestToolAction(ctx, mojoutil.ActionCreateTestService)
	if err := serviceProc.Start(); err != nil {
		s.Fatal("Failed to start test service: ", err)
	}
	pingProc := mojoutil.CreateTestToolAction(ctx, mojoutil.ActionPingTestService)
	if err := pingProc.Run(); err != nil {
		s.Fatal("Failed to ping test service: ", err)
	}
	if err := serviceProc.Kill(); err != nil {
		s.Fatal("Failed to kill test service: ", err)
	}
	serviceProc.Wait() // Clean up the resources and ignore the result.

	// sharedBufProc := createTestToolAction(ctx, actionTestSharedBuffer)
	// if err := sharedBufProc.Run(); err != nil {
	// 	s.Fatal("Failed to create shared buffer: ", err)
	// }
}
