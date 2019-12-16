// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// EmitPowerSupplyPoll emits PowerSupplyPoll D-Bus signal.
func EmitPowerSupplyPoll(ctx context.Context, msg *pmpb.PowerSupplyProperties) error {
	return emitEvent(ctx, msg, "PowerSupplyPoll")
}

// EmitSuspendImminent emits SuspendImminent D-Bus signal.
func EmitSuspendImminent(ctx context.Context, msg *pmpb.SuspendImminent) error {
	return emitEvent(ctx, msg, "SuspendImminent")
}

// EmitDarkSuspendImminent emits DarkSuspendImminent D-Bus signal.
func EmitDarkSuspendImminent(ctx context.Context, msg *pmpb.SuspendImminent) error {
	return emitEvent(ctx, msg, "DarkSuspendImminent")
}

// EmitSuspendDone emits SuspendDone D-Bus signal.
func EmitSuspendDone(ctx context.Context, msg *pmpb.SuspendDone) error {
	return emitEvent(ctx, msg, "SuspendDone")
}

func emitEvent(ctx context.Context, msg proto.Message, eventName string) error {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "unable to marshal proto to byte array")
	}

	var bytesAsStrings []string
	for _, v := range bytes {
		bytesAsStrings = append(bytesAsStrings, fmt.Sprintf("0x%02x", v))
	}

	data := "array:byte:" + strings.Join(bytesAsStrings, ",")
	args := []string{"-u", "power", "--", "dbus-send", "--sender=" + dbusInterface, "--system", "--type=signal", dbusPath, dbusInterface + "." + eventName, data}
	if err := testexec.CommandContext(ctx, "sudo", args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to emit event using dbus-send")
	}

	return nil
}
