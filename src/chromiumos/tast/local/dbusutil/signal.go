// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dbusutil provides additional functionality on top of the godbus/dbus package.
package dbusutil

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// SignalChanSize is the buffer size of channels holding signals.
	SignalChanSize = 10
)

// SignalWatcher watches for and returns D-Bus signals matched by one or more MatchSpecs.
type SignalWatcher struct {
	// Signals passes signals matched by any of the MatchSpecs passed to NewSignalWatcher.
	// This channel is buffered but must be serviced regularly; otherwise incoming
	// signals may be dropped.
	Signals chan *dbus.Signal

	conn    *dbus.Conn
	specs   []MatchSpec
	allSigs chan *dbus.Signal // all signals received by the client
}

// NewSignalWatcher returns a new SignalWatcher that will return signals on conn matched by specs.
func NewSignalWatcher(ctx context.Context, conn *dbus.Conn, specs ...MatchSpec) (*SignalWatcher, error) {
	// Add connection-level match rules to ensure that we receive the requested signals.
	// While it's not well-documented, dbus-daemon does not perform deduplication of match rules, so it's
	// safe to add the same match rule twice for two different SignalWatchers and then close one of them.
	var added []MatchSpec
	for _, spec := range specs {
		if err := conn.BusObject().CallWithContext(ctx, busInterface+".AddMatch", 0, spec.String()).Err; err != nil {
			// If we failed, remove any specs that we added.
			for _, as := range added {
				// Use context.Background in case ctx has already expired due to the test timing out.
				// dbus-daemon should never hang (and if it does, the DUT is already in bad shape).
				if err := removeMatch(context.Background(), conn.BusObject(), as); err != nil { // NOLINT
					testing.ContextLogf(ctx, "Failed to remove D-Bus match rule %q", as)
				}
			}
			return nil, err
		}
		added = append(added, spec)
	}

	sw := &SignalWatcher{
		Signals: make(chan *dbus.Signal, SignalChanSize),
		conn:    conn,
		specs:   specs,
		allSigs: make(chan *dbus.Signal, SignalChanSize),
	}

	// Matching goroutine.
	go func() {
		for sig := range sw.allSigs {
			for _, spec := range sw.specs {
				if spec.MatchesSignal(sig) {
					sw.Signals <- sig
					break
				}
			}
		}
		close(sw.Signals)
	}()
	conn.Signal(sw.allSigs)

	return sw, nil
}

// NewSignalWatcherForSystemBus is a convenience function that calls NewSignalWatcher with
// a shared connection to the system bus.
func NewSignalWatcherForSystemBus(ctx context.Context, spec ...MatchSpec) (*SignalWatcher, error) {
	// SystemBus returns a shared connection. It should not be closed.
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system bus")
	}
	return NewSignalWatcher(ctx, conn, spec...)
}

// Close stops watching for signals.
func (sw *SignalWatcher) Close(ctx context.Context) error {
	var firstErr error
	for _, spec := range sw.specs {
		// Use context.Background in case ctx has already expired due to the test timing out.
		// dbus-daemon should never hang (and if it does, the DUT is already in bad shape).
		err := removeMatch(context.Background(), sw.conn.BusObject(), spec) // NOLINT
		if firstErr == nil {
			firstErr = err
		}
	}

	// Stop retrieving from sw.conn and the matching goroutine.
	// To stop watching signals from sw.conn, i.e. sw.conn.RemoveSignal(sw.allSigs),
	// we first need to consume sw.Signals to avoid the case that sw.Signals
	// and sw.allSigs are full and sw.allSigs block conn dispatching
	// goroutines which hold RLock of its channel slice. To avoid deadlock,
	// the signal consuming goroutine must be run before RemoveSignal is
	// called, or else RemoveSignal may be blocked while acquiring Lock of
	// channel slice.
	// Then, remove sw.allSigs from conn. After returning from RemoveSignal,
	// there should be no new messages written into sw.allSigs.
	// At the end, close sw.allSigs, which lets the matching goroutine know
	// the termination.
	done := make(chan struct{})
	// The consumption goroutine will be terminated when sw.Signals is
	// closed, which is called at the last step of the matching goroutine
	// after the closing of sw.allSigs triggers its termination.
	go func() {
		for range sw.Signals {
		}
		close(done)
	}()
	sw.conn.RemoveSignal(sw.allSigs)
	close(sw.allSigs)
	// Wait for consumption goroutine.
	<-done
	return firstErr
}

// GetNextSignal returns the next signal on conn that is matched by spec.
func GetNextSignal(ctx context.Context, conn *dbus.Conn, spec MatchSpec) (*dbus.Signal, error) {
	sw, err := NewSignalWatcher(ctx, conn, spec)
	if err != nil {
		return nil, err
	}
	defer sw.Close(ctx)

	for {
		select {
		case sig := <-sw.Signals:
			return sig, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// removeMatch removes the supplied match rule from obj.
func removeMatch(ctx context.Context, obj dbus.BusObject, spec MatchSpec) error {
	return obj.CallWithContext(ctx, busInterface+".RemoveMatch", 0, spec.String()).Err
}
