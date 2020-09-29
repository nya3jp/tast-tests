// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lorgnette provides an interface to talk to lorgnette over D-Bus.
package lorgnette

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.lorgnette"
	dbusPath      = "/org/chromium/lorgnette/Manager"
	dbusInterface = "org.chromium.lorgnette.Manager"
)

// Lorgnette is used to interact with the lorgnette process over D-Bus.
// For detailed spec of each D-Bus method, please review
// src/platform2/lorgnette/dbus_bindings/org.chromium.lorgnette.Manager.xml
type Lorgnette struct {
	conn    *dbus.Conn
	obj     dbus.BusObject
	signals chan *dbus.Signal
}

// New connects to lorgnette via D-Bus and returns a Lorgnette object.
func New(ctx context.Context) (*Lorgnette, error) {
	conn, err := dbusutil.SystemBus()
	if err != nil {
		return nil, err
	}

	obj := conn.Object(dbusName, dbus.ObjectPath(dbusPath))

	return &Lorgnette{conn, obj, nil}, nil
}

// StartScan calls lorgnette's StartScan method and returns the remote response.
func (l *Lorgnette) StartScan(ctx context.Context, request *lpb.StartScanRequest) (*lpb.StartScanResponse, error) {
	marshalled, err := proto.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal StartScanRequest")
	}
	call := l.obj.CallWithContext(ctx, dbusInterface+".StartScan", 0, marshalled)
	if call.Err != nil {
		return nil, errors.Wrap(call.Err, "failed to call StartScan")
	}

	marshalled = nil
	call.Store(&marshalled)
	response := &lpb.StartScanResponse{}
	if err = proto.Unmarshal(marshalled, response); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal StartScanResponse")
	}

	return response, nil
}

// GetNextImage calls lorgnette's GetNextImage method and returns the remote response.
func (l *Lorgnette) GetNextImage(ctx context.Context, request *lpb.GetNextImageRequest, outFD uintptr) (*lpb.GetNextImageResponse, error) {
	marshalled, err := proto.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal GetNextImageRequest")
	}

	call := l.obj.CallWithContext(ctx, dbusInterface+".GetNextImage", 0, marshalled, dbus.UnixFD(outFD))
	if call.Err != nil {
		return nil, errors.Wrap(call.Err, "failed to call GetNextImage")
	}

	marshalled = nil
	call.Store(&marshalled)
	response := &lpb.GetNextImageResponse{}
	if err = proto.Unmarshal(marshalled, response); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal GetNextImageResponse")
	}

	return response, nil
}

// ListenForSignals registers 'l.signals' to receive ScanStatusChanged signals from lorgnette,
// which is needed to communicate scan completion.
func (l *Lorgnette) ListenForSignals(ctx context.Context) error {
	if err := l.conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbusPath),
		dbus.WithMatchInterface(dbusInterface),
		dbus.WithMatchMember("ScanStatusChanged"),
	); err != nil {
		return errors.Wrap(err, "failed to register for signals from lorgnette")
	}
	l.signals = make(chan *dbus.Signal, 100)
	l.conn.Signal(l.signals)
	return nil
}

// WaitForScanCompletion waits for a ScanStatusChanged signal that matches uuid
// and has a terminal status.  Intermediate statuses are ignored.  You must
// have already called l.ListenForSignals before this function will be able to
// receive any signals.
func (l *Lorgnette) WaitForScanCompletion(ctx context.Context, uuid string) error {
	for dbusSignal := range l.signals {
		var marshalled []byte
		dbus.Store(dbusSignal.Body, &marshalled)
		signal := lpb.ScanStatusChangedSignal{}
		err := proto.Unmarshal(marshalled, &signal)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal ScanStatusChangedSignal")
		}

		if signal.ScanUuid != uuid {
			continue
		}

		switch signal.State {
		case lpb.ScanState_SCAN_STATE_FAILED:
			return errors.Errorf("scan failed: %s", signal.FailureReason)
		case lpb.ScanState_SCAN_STATE_PAGE_COMPLETED:
			if signal.MorePages {
				return errors.New("did not expect additional pages for scan")
			}
		case lpb.ScanState_SCAN_STATE_COMPLETED:
			return nil
		}
	}

	return errors.New("did not receive scan completion signal")
}
