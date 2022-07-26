// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lorgnette provides an interface to talk to lorgnette over D-Bus.
package lorgnette

import (
	"context"

	"github.com/godbus/dbus/v5"
	"github.com/golang/protobuf/proto"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
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
	signals *dbusutil.SignalWatcher
}

// New connects to lorgnette via D-Bus and returns a Lorgnette object.  The
// returned object will be registered for ScanStatusChanged signals.
func New(ctx context.Context) (*Lorgnette, error) {
	conn, err := dbusutil.SystemBus()
	if err != nil {
		return nil, err
	}

	obj := conn.Object(dbusName, dbus.ObjectPath(dbusPath))

	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "ScanStatusChanged",
	}
	signals, err := dbusutil.NewSignalWatcher(ctx, conn, spec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to register for signals from lorgnette")
	}

	return &Lorgnette{conn, obj, signals}, nil
}

// StartScan calls lorgnette's StartScan method and returns the remote response.
func (l *Lorgnette) StartScan(ctx context.Context, request *lpb.StartScanRequest) (*lpb.StartScanResponse, error) {
	marshalled, err := proto.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal StartScanRequest")
	}

	var buf []byte
	if err := l.obj.CallWithContext(ctx, dbusInterface+".StartScan", 0, marshalled).Store(&buf); err != nil {
		return nil, errors.Wrap(err, "failed to call StartScan")
	}

	response := &lpb.StartScanResponse{}
	if err = proto.Unmarshal(buf, response); err != nil {
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

	var buf []byte
	if err := l.obj.CallWithContext(ctx, dbusInterface+".GetNextImage", 0, marshalled, dbus.UnixFD(outFD)).Store(&buf); err != nil {
		return nil, errors.Wrap(err, "failed to call GetNextImage")
	}

	response := &lpb.GetNextImageResponse{}
	if err = proto.Unmarshal(buf, response); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal GetNextImageResponse")
	}

	return response, nil
}

// WaitForScanCompletion waits for a ScanStatusChanged signal that matches uuid
// and has a terminal status.  Intermediate statuses are ignored.
func (l *Lorgnette) WaitForScanCompletion(ctx context.Context, uuid string) error {
	for {
		select {
		case sig := <-l.signals.Signals:
			var buf []byte
			if err := dbus.Store(sig.Body, &buf); err != nil {
				return errors.Wrap(err, "failed to extract ScanStatusChangedSignal body")
			}
			signal := lpb.ScanStatusChangedSignal{}
			if err := proto.Unmarshal(buf, &signal); err != nil {
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

		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "did not receive scan completion signal")

		}
	}
}

// StopService finds the running lorgnette process and kills it.
func StopService(ctx context.Context) error {
	return upstart.StopJob(ctx, "lorgnette")
}

// ListScanners calls lorgnette's ListScanners() method and returns the
// (possibly empty) list of ScannerInfo objects from the response.
func (l *Lorgnette) ListScanners(ctx context.Context) ([]*lpb.ScannerInfo, error) {
	var buf []byte
	if err := l.obj.CallWithContext(ctx, dbusInterface+".ListScanners", 0).Store(&buf); err != nil {
		return nil, errors.Wrap(err, "failed to call ListScanners")
	}

	response := &lpb.ListScannersResponse{}
	if err := proto.Unmarshal(buf, response); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal ListScannersResponse")
	}

	return response.Scanners, nil
}
