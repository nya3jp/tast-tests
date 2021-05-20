// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PstoreConsoleRamoops,
		Desc: "Fails if console-ramoops isn't maintained across a warm reboot",
		Contacts: []string{
			"swboyd@chromium.org",
			"chromeos-kernel-test@google.com",
		},
		SoftwareDeps: []string{"chrome", "pstore", "reboot"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// PstoreConsoleRamoops Writes a blurb to kernel logs, reboots, and checks to
// make sure console-ramoops has that message. This confirms that pstore is
// properly saving the previous kernel log.
func PstoreConsoleRamoops(ctx context.Context, s *testing.State) {
	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	cmd := `echo 'tast is rebooting' > /dev/kmsg`
	if err := d.Command("sh", "-c", cmd).Run(ctx); err != nil {
		s.Fatal("Failed to write message to /dev/kmsg on the DUT: ", err)
	}

	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	// Reinitialize gRPC connection with DUT after reboot as the current session is now stale.
	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	ramoopsPath := filepath.Join(s.OutDir(), "console-ramoops.txt")

	if err := d.GetFile(ctx, "/sys/fs/pstore/console-ramoops-0",
		ramoopsPath); err != nil {
		s.Fatal("Failed to find console-ramoops-0 after reboot on the DUT")
	}

	f, err := ioutil.ReadFile(ramoopsPath)
	if err != nil {
		s.Fatal("Failed to read ramoops file")
	}

	goodSigRegexp := regexp.MustCompile("tast is rebooting")
	if !goodSigRegexp.Match(f) {
		s.Error("Couldn't find reboot signature in console-ramoops-0")
	}
}
