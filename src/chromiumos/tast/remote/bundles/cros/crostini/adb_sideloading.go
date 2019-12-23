// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"  // remove

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const adbSideloadingBootLockboxKey = "arc_sideloading_allowed"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdbSideloading,
		Desc:         "FIXME....Verifies that system comes back after rebooting",
		Contacts:     []string{"victorhsieh@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "tpm2", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
	})
}

/*
func reboot(ctx context.Context, s *testing.State, d *dut.DUT) {
	s.Log("Rebooting DUT")
	// Run the reboot command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := "nohup sh -c 'sleep 2; reboot' >/dev/null 2>&1 </dev/null &"
	if err := d.Command("sh", "-c", cmd).Run(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	s.Log("Waiting for DUT to become unreachable")
	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("DUT became unreachable (as expected)")

	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")
}

func getAdbSideloading(ctx context.Context, d *dut.DUT) ([]byte, error) {
	cmd := d.Command("bootlockboxtool", "--action=read", "--key=arc_sideloading_allowed")
	return cmd.Output(ctx)
}

func setAdbSideloading(ctx context.Context, d *dut.DUT, enable bool) error {
	var data = "0"
	if enable {
		data = "1"
	}
	cmd := d.Command("bootlockboxtool", "--action=store", "--key=arc_sideloading_allowed", fmt.Sprintf("--data=%s", data))
	return cmd.Run(ctx)
}
*/

func AdbSideloading(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := security.NewBootLockboxServiceClient(cl.Conn)

	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{adbSideloadingBootLockboxKey})
	if err != nil {
		s.Fatal("Failed to read from boot lockbox: ", err)
	}

	fmt.Printf("response value: %s",  string(response.Value))
	// TODO do something with response

	/*
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}
	// Reset DUT to clean state
	reboot(ctx, s, d)
	if err := setAdbSideloading(ctx, d, false); err != nil {
		s.Fatal("Failed to store value in bootlockbox")
	}

	out, err := getAdbSideloading(ctx, d)
	if err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
	if string(out) != "0" {
		s.Fatal("EEEEERRRRR")
	}
	// TODO call local test

	// Reset the value
	if err := setAdbSideloading(ctx, d, false); err != nil {
		s.Fatal("Failed to store value in bootlockbox")
	}
	*/
}
