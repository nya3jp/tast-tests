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
	"chromiumos/tast/local/upstart"
)

const powerdJob = "powerd"

// PowerManagerEmitter is used to emit signals on behalf of power manager over D-Bus.
// For detailed spec of each D-Bus signal, please find
// src/platform2/power_manager/dbus_bindings/org.chromium.PowerManager.xml
type PowerManagerEmitter struct{} // NOLINT

// NewPowerManagerEmitter stops the real power manager.
func NewPowerManagerEmitter(ctx context.Context) (*PowerManagerEmitter, error) {
	if err := upstart.StopJob(ctx, powerdJob); err != nil {
		return nil, errors.Wrapf(err, "unable to stop the %s service", powerdJob)
	}

	return &PowerManagerEmitter{}, nil
}

// Stop restarts the real power manager.
func (*PowerManagerEmitter) Stop(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, powerdJob); err != nil {
		return errors.Wrapf(err, "unable to start the %s service", powerdJob)
	}
	return nil
}

// EmitPowerSupplyPoll emits PowerSupplyPoll D-Bus signal.
func (p *PowerManagerEmitter) EmitPowerSupplyPoll(ctx context.Context, msg *pmpb.PowerSupplyProperties) error {
	return p.emitEvent(ctx, msg, "PowerSupplyPoll")
}

// EmitSuspendImminent emits SuspendImminent D-Bus signal.
func (p *PowerManagerEmitter) EmitSuspendImminent(ctx context.Context, msg *pmpb.SuspendImminent) error {
	return p.emitEvent(ctx, msg, "SuspendImminent")
}

// EmitDarkSuspendImminent emits DarkSuspendImminent D-Bus signal.
func (p *PowerManagerEmitter) EmitDarkSuspendImminent(ctx context.Context, msg *pmpb.SuspendImminent) error {
	return p.emitEvent(ctx, msg, "DarkSuspendImminent")
}

// EmitSuspendDone emits SuspendDone D-Bus signal.
func (p *PowerManagerEmitter) EmitSuspendDone(ctx context.Context, msg *pmpb.SuspendDone) error {
	return p.emitEvent(ctx, msg, "SuspendDone")
}

func (*PowerManagerEmitter) emitEvent(ctx context.Context, msg proto.Message, eventName string) error {
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
