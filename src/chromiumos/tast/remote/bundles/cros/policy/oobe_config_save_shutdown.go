// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeConfigSaveShutdown,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that oobe_config_save can connect to Chrome on shutdown",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.example.ChromeService"},
		Timeout: 10 * time.Minute,
	})
}

func OobeConfigSaveShutdown(ctx context.Context, s *testing.State) {
	client, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	chromeService := example.NewChromeServiceClient(client.Conn)
	chromeService.New(ctx, &empty.Empty{})

	s.Log("clearing logs")
	if err := s.DUT().Conn().CommandContext(ctx, "sh", "-c", `echo > /var/log/messages`).Run(); err != nil {
		s.Fatal("Failed to clear logs before test: ", err)
	}

	s.Log("triggering oobe_config_save")
	if err := s.DUT().Conn().CommandContext(ctx, "touch", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		s.Fatal("Failed to initiate rollback data save: ", err)
	}

	s.Log("overriding reboot")
	if err := s.DUT().Conn().CommandContext(ctx, "update_engine_client", "--set_status=6").Run(); err != nil {
		s.Fatal("Failed to fake a pending update: ", err)
	}

	s.Log("rebooting")

	// This checks the system relaunch.
	_, _ = chromeService.RelaunchAfterUpdate(ctx, &empty.Empty{})

	// This checks the browser in-place restart.
	//_, _ = chromeService.OpenPage(ctx, &example.OpenPageRequest{Url: "chrome://restart"})

	// Wait for reboot and check logs.
	s.DUT().WaitUnreachable(ctx)
	s.DUT().Connect(ctx)

	s.Log("checking logs")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		logs, _ := s.DUT().Conn().CommandContext(ctx, "sh", "-c", `grep oobe_config_save /var/log/messages`).Output()
		if len(string(logs)) == 0 {
			return errors.New("oobe_config_save didn't run!")
		}

		logs, _ = s.DUT().Conn().CommandContext(ctx, "sh", "-c", `grep "Failed to establish dbus connection" /var/log/messages`).Output()
		if len(string(logs)) > 0 {
			return testing.PollBreak(errors.Errorf("Found Chrome connection issue: %v", string(logs)))
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed: ", err)
	}
}
