// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mojo tests the system daemon mojo_service_manager to verify the mojo
// functionality.
package mojo

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServiceManagerPolicy,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check the policy files of mojo service manager",
		Contacts: []string{
			"chromeos-mojo-service-manager@google.com",
			"chungsheng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"mojo_service_manager"},
	})
}

func ServiceManagerPolicy(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "mojo_service_manager", "--log_level=-1", "--check_policy")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Policy file validation failed: ", err)
	}
}
