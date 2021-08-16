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
	"chromiumos/tast/dut"
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
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		s.DUT().Conn().CommandContext(ctx, "stop", "oobe_config_save").Run()

		if err := s.DUT().Conn().CommandContext(ctx, "rm", "-f", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
			s.Error("Failed to remove data save flag: ", err)
		}

		if err := s.DUT().Conn().CommandContext(ctx, "rm", "-f", "/mnt/stateful_partition/rollback_data").Run(); err != nil {
			s.Error("Failed to remove rollback data: ", err)
		}

		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(cleanupCtx)

	if err := resetTPM(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
	if err := enroll(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to enroll before rollback: ", err)
	}
	if err := fakeRollback(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to fake rollback: ", err)
	}
	if err := verifyRollback(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to verify rollback: ", err)
	}
}

func resetTPM(ctx context.Context, dut *dut.DUT) error {
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, dut); err != nil {
		return errors.Wrap(err, "failed to reset TPM")
	}
	return nil
}

func enroll(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	client, err := rpc.Dial(ctx, dut, rpcHint, "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer client.Close(ctx)

	policyJSON, err := json.Marshal(fakedms.NewPolicyBlob())
	if err != nil {
		return errors.Wrap(err, "failed to serialize policies")
	}

	policyClient := ps.NewPolicyServiceClient(client.Conn)
	defer policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	if _, err := policyClient.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: policyJSON,
	}); err != nil {
		return errors.Wrap(err, "failed to enroll before rollback")
	}
	return nil
}

func fakeRollback(ctx context.Context, dut *dut.DUT) error {
	if err := dut.Conn().CommandContext(ctx, "touch", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		return errors.Wrap(err, "failed to write rollback data save file")
	}

	if err := dut.Conn().CommandContext(ctx, "start", "oobe_config_save").Run(); err != nil {
		return errors.Wrap(err, "failed to initiate saving of rollback data")
	}

	// This would be done by clobber_state during powerwash but the test does not
	// powerwash.
	if err := dut.Conn().CommandContext(ctx, "sh", "-c", `cat /var/lib/oobe_config_save/data_for_pstore > /dev/pmsg0`).Run(); err != nil {
		return errors.Wrap(err, "failed to read rollback key")
	}

	// Ineffective reset is ok here as the device steps through oobe automatically
	// which initiates retaking of TPM ownership.
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, dut); err != nil && !errors.Is(err, hwsec.ErrIneffectiveReset) {
		return errors.Wrap(err, "failed to reset TPM to fake an enterprise rollback")
	}
	return nil
}

func verifyRollback(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	client, err := rpc.Dial(ctx, dut, rpcHint, "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer client.Close(ctx)

	rollbackService := ps.NewRollbackServiceClient(client.Conn)
	response, err := rollbackService.VerifyRollback(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to verify rollback")
	}
	if !response.Successful {
		return errors.Wrap(err, "rollback was not successful")
	}
	return nil
}
