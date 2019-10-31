// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbus

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     SignalWatcherClose,
		Desc:     "Verifies dbusutil.SignalWatcher can be closed properly without deadlock when the number of signals exceeds channel buffer",
		Contacts: []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func setupPrivateSystemBus() (*dbus.Conn, error) {
	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return nil, err
	}
	if err = conn.Auth(nil); err != nil {
		conn.Close()
		return nil, err
	}
	if err = conn.Hello(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func SignalWatcherClose(ctx context.Context, s *testing.State) {
	const (
		sender = "org.example.MyService"
		path   = "/org/example/MyService"
		iface  = "org.example.MyServiceInterface"
		member = "MySignal"
		arg0   = "MyArg"
	)

	emitter, err := setupPrivateSystemBus()
	if err != nil {
		s.Fatal("Failed to create a signal emitter bus: ", err)
	}
	defer emitter.Close()

	receiver, err := setupPrivateSystemBus()
	if err != nil {
		s.Fatal("Failed to create a receiver bus: ", err)
	}
	defer receiver.Close()

	// Make the precise spec to avoid receiving unrelated signals.
	match := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbus.ObjectPath(path),
		Interface: iface,
		Member:    member,
		Arg0:      arg0,
	}

	refWatcher, err := dbusutil.NewSignalWatcherForSystemBus(ctx, match)
	if err != nil {
		s.Fatal("Failed to create a reference signal watcher: ", err)
	}
	defer refWatcher.Close(ctx)

	watcher, err := dbusutil.NewSignalWatcher(ctx, receiver, match)
	if err != nil {
		s.Fatal("Failed to create a signal watcher: ", err)
	}
	// No defer watcher.Close here because it's what we want to test.

	// Send out signals.
	sigName := iface + "." + member
	// To block receiver, we need at least emit signals of the amount:
	// (#buffers of watcher.Signals + 1 in matching goroutine) + (#buffers of watcher.allSigs + 1 in receiver)
	sigCount := 2*dbusutil.SignalChanSize + 2
	for i := 0; i < sigCount; i++ {
		emitter.Emit(path, sigName, arg0)
	}

	// Retrieve the signals from reference watcher to ensure the signals
	// have arrived. (still have some race between 2 receivers though)
	for i := 0; i < sigCount; i++ {
		select {
		case <-refWatcher.Signals:
		case <-ctx.Done():
			s.Fatal("Failed to receive all signals from the emitter in time")
		}
	}

	// Try to close the watcher. As it may hang, run it in goroutine so
	// that we can rescue it in main thread.
	closeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	closeDone := make(chan struct{})
	go func() {
		// Try to close the watcher, of which the underlying dbus.Conn,
		// receiver, is blocked by full of unread signal buffers.
		watcher.Close(closeCtx)
		// Signal main thread that we're done.
		close(closeDone)
	}()

	select {
	case <-closeDone:
		// Close finished in time.
	case <-closeCtx.Done():
		s.Error("Failed to close watcher in time")
	}

	// Cleanup.
	// Try draining out the signal to unravel deadlock if any.
	for range watcher.Signals {
	}
	// Wait for the goroutine if alive until deadline.
	select {
	case <-closeDone:
	case <-ctx.Done():
		s.Error("Failed to cleanup bg goroutine")
	}
}
