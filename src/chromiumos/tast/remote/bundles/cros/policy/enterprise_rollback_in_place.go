// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EnterpriseRollbackInPlace,
		Desc: "Check the enterprise rollback data restore mechanism while faking a rollback on one image",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.policy.RollbackService"},
		Timeout:      10 * time.Minute,
	})
}

func EnterpriseRollbackInPlace(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	defer cleanUp(ctxForCleanUp, s)

	resetTpm(ctx, s)
	enroll(ctx, s)
	fakeRollback(ctx, s)
	verifyRollback(ctx, s)
}

func resetTpm(ctx context.Context, s *testing.State) {
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
}

func enroll(ctx context.Context, s *testing.State) {
	client, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer client.Close(ctx)

	policyJSON, err := json.Marshal(fakedms.NewPolicyBlob())
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	policyClient := ps.NewPolicyServiceClient(client.Conn)
	defer policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	if _, err := policyClient.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: policyJSON,
	}); err != nil {
		s.Fatal("Failed to enroll before rollback: ", err)
	}
}

func fakeRollback(ctx context.Context, s *testing.State) {
	if err := s.DUT().Conn().Command("touch", "/mnt/stateful_partition/.save_rollback_data").Run(ctx); err != nil {
		s.Fatal("Failed to write rollback data save file: ", err)
	}

	if err := s.DUT().Conn().CommandContext(ctx, "start", "oobe_config_save").Run(); err != nil {
		s.Fatal("Failed to initiate saving of rollback data: ", err)
	}

	// This would be done by clobber_state during powerwash but the test does not
	// powerwash.
	if err := s.DUT().Conn().CommandContext(ctx, "sh", "-c", `cat /var/lib/oobe_config_save/data_for_pstore > /dev/pmsg0`).Run(); err != nil {
		s.Fatal("Failed to read rollback key: ", err)
	}

	// Ineffective reset is ok here as the device steps through oobe automatically
	// which initiates retaking of TPM ownership.
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil && !errors.Is(err, hwsec.ErrIneffectiveReset) {
		s.Fatal("Failed to reset TPM to fake an enterprise rollback: ", err)
	}
}

func verifyRollback(ctx context.Context, s *testing.State) {
	client, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer client.Close(ctx)

	rollbackService := ps.NewRollbackServiceClient(client.Conn)
	response, err := rollbackService.VerifyRollback(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to verify rollback: ", err)
	}
	if !response.RollbackSuccessful {
		s.Error("Rollback was not successful")
	}
}

func cleanUp(ctx context.Context, s *testing.State) {
	if err := s.DUT().Conn().Command("rm", "-f", "/mnt/stateful_partition/rollback_data").Run(ctx); err != nil {
		s.Error("Failed to remove rollback data: ", err)
	}

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Error("Failed to reset TPM after test: ", err)
	}
}
