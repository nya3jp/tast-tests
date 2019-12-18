// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"syscall"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/timing"
)

// ServiceOwned returns whether the service in request is already owned.
func ServiceOwned(ctx context.Context, conn *dbus.Conn, svc string) bool {
	obj := conn.Object(busName, busPath)
	return obj.CallWithContext(ctx, busInterface+".GetNameOwner", 0, svc).Err == nil
}

// WaitForService blocks until a D-Bus client on conn takes ownership of the name svc.
// If the name is already owned, it returns immediately.
func WaitForService(ctx context.Context, conn *dbus.Conn, svc string) error {
	// If the name is already owned, we're done.
	if ServiceOwned(ctx, conn, svc) {
		return nil
	}

	sw, err := NewSignalWatcher(ctx, conn, MatchSpec{
		Type:      "signal",
		Path:      busPath,
		Interface: busInterface,
		Sender:    busName,
		Member:    "NameOwnerChanged",
		Arg0:      svc,
	})
	if err != nil {
		return err
	}
	defer sw.Close(ctx)

	// Make sure the name wasn't taken while we were creating the watcher.
	if ServiceOwned(ctx, conn, svc) {
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
	ctx, st := timing.Start(ctx, fmt.Sprintf("dbusutil.Connect %s:%s", name, path))
	defer st.End()

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to system bus")
	}

	if err := WaitForService(ctx, conn, name); err != nil {
		return nil, nil, errors.Wrapf(err, "failed waiting for %s service", name)
	}

	return conn, conn.Object(name, path), nil
}

// ConnectWithAuth sets up the D-Bus connection for user with uid to the
// service specified by name, path by using SystemBusPrivate.
// This waits for the service to become available.
func ConnectWithAuth(ctx context.Context, uid uint32, name string, path dbus.ObjectPath) (*dbus.Conn, dbus.BusObject, error) {
	ctx, st := timing.Start(ctx, fmt.Sprintf("dbusutil.ConnectWithAuth %s:%s", name, path))
	defer st.End()

	currentEUID := os.Geteuid()
	runtime.LockOSThread() // See https://golang.org/issue/1435
	defer runtime.UnlockOSThread()

	if err := syscall.Setreuid(-1, int(uid)); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to set euid to %u", uid)
	}
	defer syscall.Setreuid(-1, currentEUID)

	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to system bus")
	}

	uidString := strconv.Itoa(int(uid))

	if err := conn.Auth([]dbus.Auth{dbus.AuthExternal(uidString)}); err != nil {
		conn.Close()
		return nil, nil, err
	}

	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, nil, err
	}

	if err := WaitForService(ctx, conn, name); err != nil {
		conn.Close()
		return nil, nil, errors.Wrapf(err, "failed waiting for %s service", name)
	}

	return conn, conn.Object(name, path), nil
}
