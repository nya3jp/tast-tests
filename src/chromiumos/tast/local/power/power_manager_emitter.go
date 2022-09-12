// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
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
	return p.emitEvent(ctx, msg, SignalPowerSupplyPoll)
}

// EmitSuspendImminent emits SuspendImminent D-Bus signal.
func (p *PowerManagerEmitter) EmitSuspendImminent(ctx context.Context, msg *pmpb.SuspendImminent) error {
	return p.emitEvent(ctx, msg, SignalSuspendImminent)
}

// EmitDarkSuspendImminent emits DarkSuspendImminent D-Bus signal.
func (p *PowerManagerEmitter) EmitDarkSuspendImminent(ctx context.Context, msg *pmpb.SuspendImminent) error {
	return p.emitEvent(ctx, msg, SignalDarkSuspendImminent)
}

// EmitSuspendDone emits SuspendDone D-Bus signal.
func (p *PowerManagerEmitter) EmitSuspendDone(ctx context.Context, msg *pmpb.SuspendDone) error {
	return p.emitEvent(ctx, msg, SignalSuspendDone)
}

// EmitInputEvent emits InputEvent D-Bus signal.
func (p *PowerManagerEmitter) EmitInputEvent(ctx context.Context, msg *pmpb.InputEvent) error {
	return p.emitEvent(ctx, msg, SignalInputEvent)
}

func (*PowerManagerEmitter) emitEvent(ctx context.Context, msg proto.Message, eventName string) error {
	watcher, err := NewSignalWatcher(ctx, eventName)
	if err != nil {
		return err
	}
	defer watcher.Close(ctx)

	arg, err := proto.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "unable to marshal proto to byte array")
	}

	var argAsStrings []string
	for _, v := range arg {
		argAsStrings = append(argAsStrings, fmt.Sprintf("0x%02x", v))
	}

	data := "array:byte:" + strings.Join(argAsStrings, ",")
	args := []string{"-u", "power", "--", "dbus-send", "--sender=" + dbusInterface, "--system", "--type=signal", dbusPath, dbusInterface + "." + eventName, data}

	// TODO(crbug.com/1062564): Remove polling and waiting for signals.
	return testing.Poll(ctx, func(ctx context.Context) error {
		if out, err := testexec.CommandContext(ctx, "sudo", args...).CombinedOutput(testexec.DumpLogOnError); err != nil {
			// TODO(b/246220151): Try re-stopping powerd, and waiting 1s if we get
			// error "already has owner". Either powerd is running (again), or
			// previous call to dbus-send has not released ownership.
			if strings.HasSuffix(strings.TrimSpace(string(out)), "already has owner") {
				goal, _, _, err := upstart.JobStatus(ctx, powerdJob)
				if err != nil {
					return errors.Wrap(err, "dbus-send unable unable to check job status")
				}
				testing.ContextLogf(ctx, "dbus-send failed with existing owner, will restop service in state (%s) and try again", goal)
				if err := upstart.StopJob(ctx, powerdJob); err != nil {
					return errors.Wrap(err, "dbus-send unable to restop")
				}
				testing.Sleep(ctx, time.Second)
				return errors.Wrapf(err, "dbus-send stopped service in status %s and retrying signal", goal)
			}
			return testing.PollBreak(errors.Wrapf(err, "unable to emit event using dbus-send: %s", string(out)))
		}

		select {
		case sig := <-watcher.Signals:
			// Check if arguments are identical.
			if v, ok := sig.Body[0].([]byte); !ok || !bytes.Equal(v, arg) {
				// Retry if received unexpected signal to avoid errors if received an old signal.
				return errors.Wrapf(err, "signal argument did not match: got %v; want %v", v, arg)
			}

			return nil
		case <-time.After(5 * time.Second):
			testing.ContextLog(ctx, "dbus-send failed to send signal")
			return errors.New("dbus-send failed to send signal")
		}
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}
