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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServiceManagerIntegration,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check the functionality of mojo service manager",
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

func ServiceManagerIntegration(ctx context.Context, s *testing.State) {
	const testBin = "/usr/local/libexec/mojo_service_manager/integration_tests"
	cmd := testexec.CommandContext(ctx, testBin)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Integration test failed: ", err)
	}
}
