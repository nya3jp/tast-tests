// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

const powerdJob = "powerd"

// PowerManagerEmitter is used to emit signals on behalf of power manager over D-Bus.
// For detailed spec of each D-Bus signal, please find
// src/platform2/power_manager/dbus_bindings/org.chromium.PowerManager.xml
type PowerManagerEmitter struct { // NOLINT
	conn *dbus.Conn
}

// NewPowerManagerEmitter stops real power manager, takes ownership of power
// manager D-Bus service interface and returns a PowerManagerEmitter object.
func NewPowerManagerEmitter(ctx context.Context) (*PowerManagerEmitter, error) {
	if err := upstart.StopJob(ctx, powerdJob); err != nil {
		return nil, errors.Wrapf(err, "unable to stop the %s service", powerdJob)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		upstart.RestartJob(ctx, powerdJob)
		return nil, errors.Wrap(err, "unable to connect to the dbus")
	}
	if _, err := conn.RequestName(dbusInterface, dbus.NameFlagReplaceExisting); err != nil {
		upstart.RestartJob(ctx, powerdJob)
		return nil, errors.Wrapf(err, "unable to request ownership of %s dbus name", dbusInterface)
	}

	return &PowerManagerEmitter{conn}, nil
}

// Stop relaeases D-Bus power manager service interface ownership and starts
// real power manager.
func (p *PowerManagerEmitter) Stop(ctx context.Context) error {
	if _, err := p.conn.ReleaseName(dbusInterface); err != nil {
		return errors.Wrapf(err, "unable to release %s dbus name", dbusInterface)
	}
	if err := upstart.RestartJob(ctx, powerdJob); err != nil {
		return errors.Wrapf(err, "unable to start the %s service", powerdJob)
	}
	return nil
}

// EmitPowerSupplyPoll emits PowerSupplyPoll D-Bus signal.
func (p *PowerManagerEmitter) EmitPowerSupplyPoll(msg *pmpb.PowerSupplyProperties) error {
	return p.emitEvent(msg, "PowerSupplyPoll")
}

// EmitSuspendImminent emits SuspendImminent D-Bus signal.
func (p *PowerManagerEmitter) EmitSuspendImminent(msg *pmpb.SuspendImminent) error {
	return p.emitEvent(msg, "SuspendImminent")
}

// EmitDarkSuspendImminent emits DarkSuspendImminent D-Bus signal.
func (p *PowerManagerEmitter) EmitDarkSuspendImminent(msg *pmpb.SuspendImminent) error {
	return p.emitEvent(msg, "DarkSuspendImminent")
}

// EmitSuspendDone emits SuspendDone D-Bus signal.
func (p *PowerManagerEmitter) EmitSuspendDone(msg *pmpb.SuspendDone) error {
	return p.emitEvent(msg, "SuspendDone")
}

func (p *PowerManagerEmitter) emitEvent(msg proto.Message, eventName string) error {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "unable to marshal proto to byte array")
	}
	if err := p.conn.Emit(dbus.ObjectPath(dbusPath), dbusInterface+"."+eventName, bytes); err != nil {
		return errors.Wrapf(err, "unable to emit %s event", eventName)
	}
	return nil
}
