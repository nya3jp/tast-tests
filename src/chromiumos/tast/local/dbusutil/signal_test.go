// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus"
)

func setupPrivateSystemBus() (conn *dbus.Conn, err error) {
	conn, err = dbus.SystemBusPrivate()
	if err != nil {
		return nil, err
	}
	if err = conn.Auth(nil); err != nil {
		conn.Close()
		conn = nil
		return
	}
	if err = conn.Hello(); err != nil {
		conn.Close()
		conn = nil
	}
	return conn, nil
}

func TestSignalWatcherClose(t *testing.T) {
	const (
		sender = "org.example.MyService"
		path   = "/org/example/MyService"
		iface  = "org.example.MyServiceInterface"
		member = "MySignal"
		arg0   = "MyArg"
	)

	_ = &dbus.Signal{
		Sender: sender,
		Path:   dbus.ObjectPath(path),
		Name:   iface + "." + member,
		Body:   []interface{}{arg0},
	}
	emitter, err := setupPrivateSystemBus()
	if err != nil {
		t.Fatalf("Failed to create signal emitter bus, err=%s", err.Error())
	}
	defer emitter.Close()
	receiver, err := setupPrivateSystemBus()
	if err != nil {
		t.Fatalf("Failed to create receiver bus, err=%s", err.Error())
	}
	defer receiver.Close()

	// Make the spec precise to avoid receiving unrelated signals.
	match := MatchSpec{
		Type:      "signal",
		Path:      dbus.ObjectPath(path),
		Interface: iface,
		Member:    member,
		Arg0:      arg0,
	}

	refWatcher, err := NewSignalWatcherForSystemBus(context.Background(), match)
	if err != nil {
		t.Fatalf("Failed to create signal watcher")
	}
	defer refWatcher.Close(context.Background())
	watcher, err := NewSignalWatcher(context.Background(), receiver, match)
	if err != nil {
		t.Fatalf("Failed to create signal watcher")
	}
	// No watcher.Close here because it's what we want to test.

	// Send out signals.
	sigName := iface + "." + member
	// Signals + 1 in goroutine + allSigs + 1 to obtain lock.
	sigCount := 2*signalChanSize + 2
	for i := 0; i < sigCount; i++ {
		emitter.Emit(path, sigName, arg0)
	}

	sigArrived := make(chan struct{})
	closeDone := make(chan struct{})
	go func() {
		// Retrieve the signals from reference watcher to ensure the signals
		// have arrived. (still have some race between 2 receivers though)
		for i := 0; i < sigCount; i++ {
			<-refWatcher.Signals
		}
		close(sigArrived)
		// Now try close a watcher with channel filled + more incoming event.
		watcher.Close(context.Background())
		// Signal main thread that we're done.
		close(closeDone)
	}()
	select {
	case <-sigArrived:
	case <-time.After(10 * time.Second):
		t.Fatal("Fail to receive all sent signals")
	}
	select {
	case <-closeDone:
	case <-time.After(10 * time.Second):
		t.Error("Fail to close watcher in time")
	}
	// Drain out the signal to unravel deadlock.
	for range watcher.Signals {
	}
	// Cleanup go routine.
	<-closeDone
}
