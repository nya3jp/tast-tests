// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RPCDUTReboot,
		Desc: "Demonstrates how RPCDUT maintains an RPC connection across DUT reboots",
		Contacts: []string{
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:        []string{"group:mainline"},
		ServiceDeps: []string{dutfs.ServiceName},
	})
}

func readBootTime(ctx context.Context, d *rpcdut.RPCDUT) (int64, error) {
	fs := dutfs.NewClient(d.RPC().Conn)
	bytes, err := fs.ReadFile(ctx, "/proc/stat")
	if err != nil {
		return 0, err
	}

	// Search for "btime 1628806576".
	for _, line := range strings.Split(string(bytes), "\n") {
		parts := strings.Split(line, " ")
		if len(parts) == 2 {
			if parts[0] == "btime" {
				return strconv.ParseInt(parts[1], 10, 64)
			}
		}
	}

	return 0, errors.New("failed to find boot time")
}

// RPCDUTReboot demonstrates how you'd use RPCDUT.
func RPCDUTReboot(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	// We examine the error in this example simply to expose issues with Close.
	defer func(ctx context.Context) {
		if err := d.Close(ctx); err != nil {
			s.Fatal("Failed to close RPCDUT: ", err)
		}
	}(ctx)

	bootTimeBefore, err := readBootTime(ctx, d)
	if err != nil {
		s.Fatal("Failed to read boot time: ", err)
	}
	testing.ContextLog(ctx, "Boot time before reboot: ", bootTimeBefore)

	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot dut: ", err)
	}

	bootTimeAfter, err := readBootTime(ctx, d)
	if err != nil {
		s.Fatal("Failed to read boot time: ", err)
	}
	testing.ContextLog(ctx, "Boot time after reboot: ", bootTimeAfter)

	if bootTimeBefore >= bootTimeAfter {
		s.Fatal("We failed to reboot")
	}

}
