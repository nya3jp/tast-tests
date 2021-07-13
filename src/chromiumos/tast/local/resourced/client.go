// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package resourced

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusInterface = "org.chromium.ResourceManager"
	dbusPath      = "/org/chromium/ResourceManager"
	dbusService   = "org.chromium.ResourceManager"

	GameModeOff      = 0
	GameModeBorealis = 1
)

type Client struct {
	obj *dbusutil.DBusObject
}

func (c *Client) GameMode(ctx context.Context) (uint8, error) {
	var result uint8
	if err := c.obj.Call(ctx, "GetGameMode").Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetGameMode")
	}
	return result, nil
}

func (c *Client) SetGameMode(ctx context.Context, mode uint8) error {
	if err := c.obj.Call(ctx, "SetGameMode", mode).Err; err != nil {
		return errors.Wrap(err, "failed to call method SetGameMode")
	}
	return nil
}

func (c *Client) AvailableMemoryKB(ctx context.Context) (uint64, error) {
	var result uint64
	if err := c.obj.Call(ctx, "GetAvailableMemoryKB").Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetAvailableMemoryKB")
	}
	return result, nil
}

func (c *Client) ForegroundAvailableMemoryKB(ctx context.Context) (uint64, error) {
	var result uint64
	if err := c.obj.Call(ctx, "GetForegroundAvailableMemoryKB").Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetForegroundAvailableMemoryKB")
	}
	return result, nil
}

type Margins struct {
	ModerateKB, CriticalKB uint64
}

func (c *Client) MemoryMarginsKB(ctx context.Context) (Margins, error) {
	var m Margins
	if err := c.obj.Call(ctx, "GetMemoryMarginsKB").Store(&m.CriticalKB, &m.ModerateKB); err != nil {
		return m, errors.Wrap(err, "failed to call method GetMemoryMarginsKB")
	}
	return m, nil
}

type PressureSignal struct {
	Level uint8
	Delta uint64
}

func parsePressureSignal(sig *dbus.Signal) (PressureSignal, error) {
	if len(sig.Body) != 2 {
		return PressureSignal{}, errors.Errorf("expected 2 params, got %d", len(sig.Body))
	}
	level, ok := sig.Body[0].(uint8)
	if !ok {
		return PressureSignal{}, errors.Errorf("unable to convert level from %v", sig.Body[0])
	}
	delta, ok := sig.Body[1].(uint64)
	if !ok {
		return PressureSignal{}, errors.Errorf("unable to convert delta from %v", sig.Body[1])
	}
	return PressureSignal{
		Level: level,
		Delta: delta,
	}, nil
}

type PressureWatcher struct {
	watcher *dbusutil.SignalWatcher
	err     error
	Signals chan PressureSignal
}

func (pw *PressureWatcher) Close(ctx context.Context) error {
	if err := pw.watcher.Close(ctx); err != nil {
		if pw.err != nil {
			// Log the error, we will return the earlier error.
			testing.ContextLog(ctx, "Failed to close watcher after another failure: ", err)
		}
	}
	return pw.err
}

func (c *Client) NewPressureWatcher(ctx context.Context) (*PressureWatcher, error) {
	pw := &PressureWatcher{}
	var err error
	pw.watcher, err = dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "MemoryPressureChrome",
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to watch MemoryPressureChrome signal")
	}
	// Map DBUS signals into PressureSignal.
	pw.Signals = make(chan PressureSignal)
	go func() {
		for dbusSig := range pw.watcher.Signals {
			pressureSig, err := parsePressureSignal(dbusSig)
			if err != nil {
				// NB: set err before close, so it will be set by the time a
				// consumer unblocks.
				pw.err = err
				close(pw.Signals)
				return
			}
			pw.Signals <- pressureSig
		}
		close(pw.Signals)
	}()
	return pw, nil
}

func NewClient(ctx context.Context) (*Client, error) {
	obj, err := dbusutil.NewDBusObject(ctx, dbusService, dbusInterface, dbusPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Resource Manager")
	}
	return &Client{obj}, nil
}
