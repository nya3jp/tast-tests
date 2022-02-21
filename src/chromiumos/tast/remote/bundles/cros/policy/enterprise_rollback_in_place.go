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
	aupb "chromiumos/tast/services/cros/autoupdate"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnterpriseRollbackInPlace,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check the enterprise rollback data restore mechanism while faking a rollback on one image",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.policy.PolicyService",
			"tast.cros.autoupdate.RollbackService",
		},
		Timeout: 10 * time.Minute,
	})
}

var logsAndCrashes = []string{"/var/log", "/var/spool/crash", "/home/chronos/crash", "/mnt/stateful_partition/unencrypted/preserve/crash", "/run/crash_reporter/crash"}

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
		s.Fatal("Failed to reset TPM before test: ", err)
	}
	if err := enroll(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to enroll before rollback: ", err)
	}
	guid, err := configureNetwork(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to configure network: ", err)
	}
	if err := saveRollbackData(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to save rollback data: ", err)
	}

	sensitive, err := sensitiveDataForPstore(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to read sensitive data for pstore: ", err)
	}

	// Ineffective reset is ok here as the device steps through oobe automatically
	// which initiates retaking of TPM ownership.
	if err := resetTPM(ctx, s.DUT()); err != nil && !errors.Is(err, hwsec.ErrIneffectiveReset) {
		s.Fatal("Failed to reset TPM to fake an enterprise rollback: ", err)
	}

	if err := ensureSensitiveDataIsNotLogged(ctx, s.DUT(), sensitive); err != nil {
		s.Fatal("Failed while checking that sensitive data is not logged: ", err)
	}

	if err := verifyRollback(ctx, guid, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to verify rollback: ", err)
	}
}

func resetTPM(ctx context.Context, dut *dut.DUT) error {
	return policyutil.EnsureTPMAndSystemStateAreReset(ctx, dut)
}

func enroll(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	client, err := rpc.Dial(ctx, dut, rpcHint)
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

func configureNetwork(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) (string, error) {
	client, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer client.Close(ctx)

	rollbackService := aupb.NewRollbackServiceClient(client.Conn)
	response, err := rollbackService.SetUpPskNetwork(ctx, &empty.Empty{})
	if err != nil {
		return "", errors.Wrap(err, "failed to configure psk network on client")
	}
	return response.Guid, nil
}

func saveRollbackData(ctx context.Context, dut *dut.DUT) error {
	if err := dut.Conn().CommandContext(ctx, "touch", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		return errors.Wrap(err, "failed to write rollback data save file")
	}

	if err := dut.Conn().CommandContext(ctx, "start", "oobe_config_save").Run(); err != nil {
		return errors.Wrap(err, "failed to run oobe_config_save")
	}

	// The following two commands would be done by clobber_state during powerwash
	// but the test does not powerwash.
	if err := dut.Conn().CommandContext(ctx, "sh", "-c", `cat /var/lib/oobe_config_save/data_for_pstore > /dev/pmsg0`).Run(); err != nil {
		return errors.Wrap(err, "failed to read rollback key")
	}
	// Adds a newline to pstore.
	if err := dut.Conn().CommandContext(ctx, "sh", "-c", `echo "" >> /dev/pmsg0`).Run(); err != nil {
		return errors.Wrap(err, "failed to add newline after rollback key")
	}

	return nil
}

func sensitiveDataForPstore(ctx context.Context, dut *dut.DUT) (string, error) {
	sensitive, err := dut.Conn().CommandContext(ctx, "cat", "/var/lib/oobe_config_save/data_for_pstore").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to read data_for_pstore")
	}
	return string(sensitive), nil
}

func ensureSensitiveDataIsNotLogged(ctx context.Context, dut *dut.DUT, sensitive string) error {
	for _, folder := range logsAndCrashes {
		err := dut.Conn().CommandContext(ctx, "grep", "-rq", sensitive, folder).Run()
		if err == nil {
			return errors.Errorf("sensitive data found by grep in folder %q", folder)
		}
	}
	return nil
}

func verifyRollback(ctx context.Context, guid string, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	client, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer client.Close(ctx)

	rollbackService := aupb.NewRollbackServiceClient(client.Conn)
	response, err := rollbackService.VerifyRollback(ctx, &aupb.VerifyRollbackRequest{Guid: guid})
	if err != nil {
		return errors.Wrap(err, "failed to verify rollback on client")
	}
	if !response.Successful {
		return errors.Wrapf(err, "rollback was not successful: %s", response.VerificationDetails)
	}

	return nil
}
