// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/factory/toolkit"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Writeprotect,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test write protect ability in factory toolkit",
		Contacts:     []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Timeout:      time.Minute,
		Fixture:      fixture.DevModeGBB,
		SoftwareDeps: []string{"factory_flow"},
		ServiceDeps:  []string{toolkit.ToolkitServiceDep},
	})
}

func Writeprotect(ctx context.Context, s *testing.State) {
	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Failed to require Servo: ", err)
	}

	disableHardwareWriteProtectForTesting(ctx, firmwareHelper, s)
	defer restoringHardwareWriteProtectAfterTesting(ctx, firmwareHelper, s)

	installFactorySoftwareForTesting(ctx, s)
	defer uninstallFactorySoftwareAfterTesting(ctx, s)

	testGooftoolAPWriteProtect(ctx, "disable", nil, false, s)
	testGooftoolAPWriteProtect(ctx, "enable", []string{"--skip_enable_check"}, true, s)
}

func disableHardwareWriteProtectForTesting(ctx context.Context, h *firmware.Helper, s *testing.State) {
	if err := setHardwareWriteProtect(ctx, h, false); err != nil {
		s.Fatal("Failed to disable hardware write protect for testing: ", err)
	}
}

func restoringHardwareWriteProtectAfterTesting(ctx context.Context, h *firmware.Helper, s *testing.State) {
	if err := setHardwareWriteProtect(ctx, h, true); err != nil {
		s.Fatal("Failed to restore hardware write protect after testing: ", err)
	}
}

func installFactorySoftwareForTesting(ctx context.Context, s *testing.State) {
	if _, err := toolkit.InstallFactoryToolKit(ctx, s.DUT(), s.RPCHint(), true); err != nil {
		s.Fatal("Failed to prepare gooftool: ", err)
	}
}

func uninstallFactorySoftwareAfterTesting(ctx context.Context, s *testing.State) {
	if err := toolkit.UninstallFactoryToolKit(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to restore DUT status for uninstalling factory toolkit: ", err)
	}
}

func testGooftoolAPWriteProtect(ctx context.Context, operation string, extraArgs []string, expectedEnabled bool, s *testing.State) {
	args := append([]string{
		"write_protect",
		"--operation",
		operation,
		"--targets",
		"AP",
	}, extraArgs...)
	conn := s.DUT().Conn()
	if _, err := conn.CommandContext(ctx, "gooftool", args...).Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to use gooftool to set write protect state: ", err)
	}

	enabled, err := getAPWriteProtectState(ctx, conn)
	if err != nil {
		s.Fatal("Failed to get write protect state with gooftool: ", err)
	}
	if enabled != expectedEnabled {
		s.Fatal("Write protect state does not match to the state after set")
	}
}

func getAPWriteProtectState(ctx context.Context, conn *ssh.Conn) (bool, error) {
	cmd := conn.CommandContext(ctx, "gooftool", "write_protect", "--operation", "show")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return false, err
	}

	responseReader := bytes.NewReader(output)
	decoder := json.NewDecoder(responseReader)
	var gooftoolShowResponse gooftoolShowResult
	if err := decoder.Decode(&gooftoolShowResponse); err != nil {
		return false, errors.Wrap(err, "failed to parse show result")
	}
	return gooftoolShowResponse.AP.Enabled, nil
}

func setHardwareWriteProtect(ctx context.Context, h *firmware.Helper, enable bool) error {
	desiredState := servo.FWWPStateOff
	if enable {
		desiredState = servo.FWWPStateOn
	}

	if err := h.Servo.SetFWWPState(ctx, desiredState); err != nil {
		if enable {
			return errors.Wrap(err, "failed to enable firmware write protect")
		}
		return errors.Wrap(err, "failed to disable firmware write protect")
	}
	return nil
}

type gooftoolShowResult struct {
	AP *writeprotectState `json:"AP"`
}

type writeprotectState struct {
	Enabled bool `json:"enabled"`
	Offset  uint `json:"offset"`
	Size    uint `json:"size"`
}
