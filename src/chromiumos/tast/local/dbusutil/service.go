// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// WaitForService blocks until a D-Bus client on conn takes ownership of the name svc.
// If the name is already owned, it returns immediately.
func WaitForService(ctx context.Context, conn *dbus.Conn, svc string) error {
	obj := conn.Object(BusName, BusPath)
	owned := func() bool { return obj.CallWithContext(ctx, BusInterface+".GetNameOwner", 0, svc).Err == nil }

	// If the name is already owned, we're done.
	if owned() {
		return nil
	}

	sw, err := NewSignalWatcher(ctx, conn, MatchSpec{
		Type:      "signal",
		Path:      BusPath,
		Interface: BusInterface,
		Sender:    BusName,
		Member:    "NameOwnerChanged",
		Arg0:      svc,
	})
	if err != nil {
		return err
	}
	defer sw.Close(ctx)

	// Make sure the name wasn't taken while we were creating the watcher.
	if owned() {
		return nil
	}

	for {
		select {
		case sig := <-sw.Signals:
			if len(sig.Body) < 3 {
				continue
			}
			// Skip signals about this service if the "new owner" arg is empty.
			if v, ok := sig.Body[2].(string); !ok || v == "" {
				continue
			}
			// Otherwise, we're done.
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Connect sets up the D-Bus connection to the service specified by name,
// path by using SystemBus.
// This waits for the service to become available.
func Connect(ctx context.Context, name string, path dbus.ObjectPath) (*dbus.Conn, dbus.BusObject, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to system bus")
	}

	testing.ContextLogf(ctx, "Waiting for %s D-Bus service", name)
	if err := WaitForService(ctx, conn, name); err != nil {
		return nil, nil, errors.Wrapf(err, "failed waiting for %s service", name)
	}

	return conn, conn.Object(name, path), nil
}
