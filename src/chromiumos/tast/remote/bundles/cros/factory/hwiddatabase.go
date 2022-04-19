// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/factory/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var modelsNotProducing = []string{"asuka", "bob", "banon", "caroline", "dru", "dumo", "edgar", "kefka", "kevin", "lars", "reks", "relm", "sentry", "terra", "ultima", "wizpig"}

func init() {
	testing.AddTest(&testing.Test{
		Func:         HWIDDatabase,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test the flow from collecting materials, generating HWID database, to verifing the database is valid",
		Contacts:     []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      2 * time.Minute,
		Fixture:      fixture.EnsureToolkit,
		HardwareDeps: hwdep.D(
			hwdep.SkipOnModel(modelsNotProducing...),
		),
		SoftwareDeps: append([]string{"factory_flow"}, fixture.EnsureToolkitSoftwareDeps...),
	})
}

func HWIDDatabase(ctx context.Context, s *testing.State) {
	conn := s.DUT().Conn()
	buildDatabaseCmd := conn.CommandContext(ctx, "hwid", "build-database")
	if err := buildDatabaseCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to build the HWID database: ", err)
	}

	// Only verifies the format, probed components. As the database is built
	// locally and checksum is not the testing target, skip the check of
	// checksum here.
	verifyDatabaseCmd := conn.CommandContext(ctx, "hwid", "verify-database", "--no-verify-checksum")
	if err := verifyDatabaseCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Verify built HWID database failed: ", err)
	}
}
