// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/exec"
	"chromiumos/tast/remote/bundles/cros/audio/internal"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileOwnershipMigration,
		Desc:         "Check files are migrated on boot",
		SoftwareDeps: []string{"cras", "reboot"},
		Contacts:     []string{"aaronyu@google.com", "chromeos-audio-sw@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
	})
}

func FileOwnershipMigration(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}

	// Ensure the rpc connection is closed at the end of this test.
	defer func(ctx context.Context) {
		if err := d.Close(ctx); err != nil {
			s.Fatal("Failed to close RPCDUT: ", err)
		}
	}(cleanupCtx)

	// Install file with incorrect ownership.
	cmd := d.Conn().CommandContext(ctx,
		"install", "-Tm644", "-oroot", "-groot", "/dev/null", internal.CrasStopTimeFile)
	if err := cmd.Run(exec.DumpLogOnError); err != nil {
		s.Fatalf("Error creating %s: %s", internal.CrasStopTimeFile, err)
	}

	// Reboot should migrate file ownership.
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	cmd = d.Conn().CommandContext(ctx, "stat", "--format=%U,%G,%a", internal.CrasStopTimeFile)
	stdout, err := cmd.Output(exec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to stat %s: %s", internal.CrasStopTimeFile, err)
	}

	const wantStat = "cras,cras,644"
	gotStat := strings.TrimRight(string(stdout), "\n")
	if gotStat != wantStat {
		s.Fatalf("Unexpected user,group,mode. Want %q; got %q", wantStat, gotStat)
	}
}
