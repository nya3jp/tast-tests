// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hibernate

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/hibernate/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HibermanDryRun,
		Desc: "Checks the hiberman logs to ensure correct hibernate flow",
		Contacts: []string{
			"chromeos-hibernate@google.com", // Test owners
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// HibermanDryRun executes the hibernate dry-run and checks the log correctness.
func HibermanDryRun(ctx context.Context, s *testing.State) {
	const (
		// hibernationDeadline is an approximate time required to hibernate the machine.
		// The deadline is quite generous, as the time depends on a machine performance
		// and RAM-to-disk image size.
		hibernationDeadline = 60 * time.Second
	)

	s.Log("Start utils.StreamLogs()")
	logCmd, logCh, err := utils.StreamLogs(ctx)
	if err != nil {
		s.Fatal("Failed to start utils.StreamLogs(): ", err)
	}
	defer logCmd.Wait()
	defer logCmd.Kill()

	cmd := []string{"hiberman", "hibernate", "--test-keys", "--dry-run"}
	_, err = testexec.CommandContext(ctx, cmd[0], cmd[1:]...).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to execute %v: %v", cmd, err)
	}

	// Expected order of logs from correctly functioning hibernate dry-run.
	// TODO(b/237084693): Request from evgreen@ in crrev/c/3992310 is to migrate
	// from using human readable logs to telemetry metrics.
	var hibermanHibernateLogs = []string{
		"Beginning hibernate",
		"Mounting hibervol",
		"Wrote hibernate image in",
		"Setting hibernate cookie at",
	}
	for _, expMsg := range hibermanHibernateLogs {
		s.Logf("Watching for %q in logs", expMsg)
		ctx, cancel := context.WithTimeout(ctx, hibernationDeadline)
		defer cancel()
		if err := utils.DetectLog(ctx, s, logCh, expMsg); err != nil {
			s.Fatal("Failed to execute utils.DetectLog(): ", err)
		}
	}
}
