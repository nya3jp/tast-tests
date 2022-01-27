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
	"sync"
	"syscall"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/timing"
)

var (
	systemBus    *dbus.Conn
	systemBusMux sync.Mutex
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

// busOptions returns the common bus options that should be used for all
// DBus connections.
func busOptions() []dbus.ConnOption {
	// Enforce sequential processing of all signals. This means applications
	// listening for signals will receive them on in the order that they are
	// received on the dbus connection, rather than in an arbitrary order
	// (as is dbus's default, for arcane reasons). This is necessary in order
	// to sensibly implement things like state change listeners, where one expects
	// the most recently received signal to represent the most recent state.
	return []dbus.ConnOption{
		dbus.WithSignalHandler(dbus.NewSequentialSignalHandler()),
	}
}

// SystemBus returns a shared connection to the system bus, connecting to it if
// it is not already connected. It should be used in preference to dbus.SystemBus().
func SystemBus() (conn *dbus.Conn, err error) {
	systemBusMux.Lock()
	defer systemBusMux.Unlock()
	if systemBus != nil {
		return systemBus, nil
	}
	conn, err = dbus.ConnectSystemBus(busOptions()...)
	if conn != nil {
		systemBus = conn
	}
	return
}

// SystemBusPrivate returns a new private connection to the system bus.
// It should be used in preference to dbus.SystemBusPrivate().
func SystemBusPrivate(opts ...dbus.ConnOption) (*dbus.Conn, error) {
	// Append user-passed options after our default options, so that
	// they can override our defaults.
	return dbus.SystemBusPrivate(append(busOptions(), opts...)...)
}

// Connect sets up the D-Bus connection to the service specified by name, path by using SystemBus.
// This waits for the service to become available but does not validate path existence.
func Connect(ctx context.Context, name string, path dbus.ObjectPath) (*dbus.Conn, dbus.BusObject, error) {
	ctx, st := timing.Start(ctx, fmt.Sprintf("dbusutil.Connect %s:%s", name, path))
	defer st.End()

	return ConnectNoTiming(ctx, name, path)
}

// ConnectNoTiming, like Connect() but without emitting timing information.
func ConnectNoTiming(ctx context.Context, name string, path dbus.ObjectPath) (*dbus.Conn, dbus.BusObject, error) {
	conn, err := SystemBus()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to system bus")
	}

	if err := WaitForService(ctx, conn, name); err != nil {
		return nil, nil, errors.Wrapf(err, "failed waiting for %s service", name)
	}

	return conn, conn.Object(name, path), nil
}

// SystemBusPrivateWithAuth returns a connection with switched euid.
// The returned *dbus.Conn should be closed after use.
func SystemBusPrivateWithAuth(ctx context.Context, uid uint32) (*dbus.Conn, error) {
	ignoreRUID := -1
	origEUID := os.Geteuid()

	// Workaround for b/206462481: the syscall.Setreuid attempts to change the
	// EUID on all threads and hangs.  Instead, we change the EUID on current OS
	// thread only (via syscall.RawSyscall) and ensure that this same thread is
	// used to communicate with DBus by calling runtime.LockOSThread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if _, _, err := syscall.RawSyscall(syscall.SYS_SETREUID, uintptr(ignoreRUID), uintptr(uid), 0); err != 0 {
		return nil, errors.Wrapf(err, "failed to set euid to %d", uid)
	}
	defer syscall.RawSyscall(syscall.SYS_SETREUID, uintptr(ignoreRUID), uintptr(origEUID), 0)

	conn, err := dbus.SystemBusPrivate(busOptions()...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system bus")
	}

	uidString := strconv.Itoa(int(uid))

	if err := conn.Auth([]dbus.Auth{dbus.AuthExternal(uidString)}); err != nil {
		conn.Close()
		return nil, err
	}

	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// ConnectPrivateWithAuth sets up the D-Bus connection for user with uid to the
// service specified by name, path by using SystemBusPrivate.  And like
// SystemBusPrivateWithAuth, the connection should be closed after use.
// This waits for the service to become available.
func ConnectPrivateWithAuth(ctx context.Context, uid uint32, name string, path dbus.ObjectPath) (*dbus.Conn, dbus.BusObject, error) {
	ctx, st := timing.Start(ctx, fmt.Sprintf("dbusutil.ConnectPrivateWithAuth %s:%s", name, path))
	defer st.End()

	conn, err := SystemBusPrivateWithAuth(ctx, uid)
	if err != nil {
		return nil, nil, err
	}

	if err := WaitForService(ctx, conn, name); err != nil {
		conn.Close()
		return nil, nil, errors.Wrapf(err, "failed waiting for %s service", name)
	}

	return conn, conn.Object(name, path), nil
}
