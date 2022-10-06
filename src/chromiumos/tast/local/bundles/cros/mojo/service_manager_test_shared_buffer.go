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
		Func:         ServiceManagerTestSharedBuffer,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check we can create shared buffer from mojo service manager",
		Contacts: []string{
			"chromeos-mojo-service-manager@google.com",
			"chungsheng@google.com",
		},
		Attr:    []string{"group:mainline"},
		Timeout: 1 * time.Minute,
	})
}

// ServiceManagerTestSharedBuffer tests the shared buffer can be created.
func ServiceManagerTestSharedBuffer(ctx context.Context, s *testing.State) {
	sharedBufProc := testexec.CommandContext(ctx, constants.TestToolBinary, "--action=test-shared-buffer")
	if err := sharedBufProc.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create shared buffer: ", err)
	}
}
