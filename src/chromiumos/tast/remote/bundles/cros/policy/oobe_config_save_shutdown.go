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
	// If the device hasn't been rebooted since the last time the test ran we need to remove
	// the flag indicating a successful run of oobe_config_save. This command will fail if the file
	// does not exist which is fine.
	s.DUT().Conn().CommandContext(ctx, "rm", "/var/lib/oobe_config_save/.data_saved").Run()

	s.Log("placing trigger to run oobe_config_save on shutdown")
	if err := s.DUT().Conn().CommandContext(ctx, "touch", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		s.Fatal("Failed to initiate rollback data save: ", err)
	}

	// Setting up Chrome will restart ui job which would trigger a reboot once update is pending.
	// Thus, already set it up here.
	s.Log("logging in")
	client, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	service := autoupdate.NewUpdateUIServiceClient(client.Conn)
	if _, err = service.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to click relaunch on the test device: ", err)
	}

	s.Log("pretending an update is pending")
	if err := s.DUT().Conn().CommandContext(ctx, "update_engine_client", "--set_status=6").Run(); err != nil {
		s.Fatal("Failed to fake a pending update: ", err)
	}

	s.Log("clicking restart button")
	if _, err = service.RelaunchAfterUpdate(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to click relaunch on the test device: ", err)
	}

	s.Log("waiting for the test device to become unreachable")
	if err := s.DUT().WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable during reboot: ", err)
	}

	if err := s.DUT().Connect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT after relaunch: ", err)
	}

	s.Log("checking that oobe_config_save ran successfully")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		logs, _ := s.DUT().Conn().CommandContext(ctx, "sh", "-c", `grep "ERR oobe_config_save" /var/log/messages`).Output() // TODO this will silently stop working if the way erros are logged changes. Can we do better?
		if len(string(logs)) > 0 {
			return testing.PollBreak(errors.Errorf("found oobe_config_save issue in the logs: %v", string(logs)))
		}

		if err = s.DUT().Conn().CommandContext(ctx, "sh", "-c", `test -f /var/lib/oobe_config_save/.data_saved`).Run(); err != nil {
			return errors.New("oobe_config_save didn't run")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed: ", err)
	}
}
