// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeConfigSaveShutdown,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that oobe_config_save runs successfully on reboot",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.UpdateUIService"},
		Timeout: 7 * time.Minute,
	})
}

func OobeConfigSaveShutdown(ctx context.Context, s *testing.State) {
	// Setting up Chrome will restart ui job which would trigger a reboot once update is pending.
	// Thus, already set it up here.
	s.Log("Connecting to client")
	client, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	s.Log("Logging into Chrome testing")
	service := autoupdate.NewUpdateUIServiceClient(client.Conn)
	if _, err = service.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed log into Chrome: ", err)
	}

	s.Log("Adding data save flag")
	if err := s.DUT().Conn().CommandContext(ctx, "touch", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		s.Fatal("Failed to initiate rollback data save: ", err)
	}
	// Attempt to remove the flag after leaving the test.
	defer func(ctx context.Context) {
		s.DUT().Conn().CommandContext(ctx, "rm", "/mnt/stateful_partition/.save_rollback_data").Run()
	}(ctx)

	s.Log("Faking pending update")
	if err := s.DUT().Conn().CommandContext(ctx, "update_engine_client", "--set_status=6").Run(); err != nil {
		s.Fatal("Failed to fake a pending update: ", err)
	}
	// Restart update-engine after leaving the test.
	defer func(ctx context.Context) {
		s.DUT().Conn().CommandContext(ctx, "restart", "update-engine").Run()
	}(ctx)

	s.Log("Clicking relaunch button")
	if _, err = service.RelaunchAfterUpdate(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to click relaunch on the test device: ", err)
	}

	if err := s.DUT().WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable during reboot: ", err)
	}

	if err := s.DUT().WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT after relaunch: ", err)
	}

	s.Log("Waiting for log files and data save flag")
	// Need to poll here because it may take a bit longer for content in /var to be readable after relaunch.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Note that this check relies on log format in /var/log/messages.
		logs, _ := s.DUT().Conn().CommandContext(ctx, "sh", "-c", `grep --text "INFO oobe_config_save" /var/log/messages`).Output()
		if len(string(logs)) == 0 {
			return errors.New("found no indication of oobe_config_save running in /var/log/messages")
		}

		logs, _ = s.DUT().Conn().CommandContext(ctx, "sh", "-c", `grep --text "ERR oobe_config_save" /var/log/messages`).Output()
		if len(string(logs)) > 0 {
			return testing.PollBreak(errors.Errorf("found oobe_config_save issue in the logs: %v", string(logs)))
		}

		if err := s.DUT().Conn().CommandContext(ctx, "rm", "/var/lib/oobe_config_save/.data_saved").Run(); err != nil {
			return errors.Wrap(err, "could not remove flag file indicating a successful run of oobe_config_save, probably because it did not run or finish successfully")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Something went wrong with running oobe_config_save on shutdown: ", err)
	}
}
