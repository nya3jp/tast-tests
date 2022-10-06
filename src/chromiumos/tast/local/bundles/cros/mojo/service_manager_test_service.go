// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mojo tests the system daemon mojo_service_manager to verify the mojo
// functionality.
package mojo

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/mojo/constants"
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
		Attr:    []string{"group:mainline"},
		Timeout: 1 * time.Minute,
	})
}

// ServiceManagerTestService checks whether test tool can register and request a
// test service.
func ServiceManagerTestService(ctx context.Context, s *testing.State) {
	serviceProc := testexec.CommandContext(ctx, constants.TestToolBinary, "--action=create-test-service")
	if err := serviceProc.Start(); err != nil {
		s.Fatal("Failed to start test service: ", err)
	}
	defer func() {
		if err := serviceProc.Kill(); err != nil {
			s.Fatal("Failed to kill test service: ", err)
		}
		serviceProc.Wait() // Clean up the resources and ignore the result.
	}()

	pingProc := testexec.CommandContext(ctx, constants.TestToolBinary, "--action=ping-test-service")
	if err := pingProc.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to ping test service: ", err)
	}
}
