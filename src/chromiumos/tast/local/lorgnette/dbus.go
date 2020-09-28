// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lorgnette provides an interface to talk to lorgnette over D-Bus.
package lorgnette

import (
	"context"
	"syscall"
	"time"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
	"github.com/shirou/gopsutil/process"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
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

// New connects to lorgnette via D-Bus and returns a Lorgnette object.  The
// returned object will be registered for ScanStatusChanged signals.
func New(ctx context.Context) (*Lorgnette, error) {
	conn, err := dbusutil.SystemBus()
	if err != nil {
		return nil, err
	}

	obj := conn.Object(dbusName, dbus.ObjectPath(dbusPath))

	if err = conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbusPath),
		dbus.WithMatchInterface(dbusInterface),
		dbus.WithMatchMember("ScanStatusChanged"),
	); err != nil {
		return nil, errors.Wrap(err, "failed to register for signals from lorgnette")
	}
	signals := make(chan *dbus.Signal, 100)
	conn.Signal(signals)

	return &Lorgnette{conn, obj, signals}, nil
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

// WaitForScanCompletion waits for a ScanStatusChanged signal that matches uuid
// and has a terminal status.  Intermediate statuses are ignored.
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

// Kill finds the running lorgnette process and kills it.
func Kill(ctx context.Context) error {
	ps, err := process.Processes()
	if err != nil {
		return err
	}

	for _, p := range ps {
		if name, err := p.Name(); err != nil || name != "lorgnette" {
			continue
		}

		if err := syscall.Kill(int(p.Pid), syscall.SIGINT); err != nil && err != syscall.ESRCH {
			return errors.Wrap(err, "failed to kill lorgnette")
		}

		// Wait for the process to exit so that its sockets can be removed.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// We need a fresh process.Process since it caches attributes.
			// TODO(crbug.com/1131511): Clean up error handling here when gpsutil has been upreved.
			if _, err := process.NewProcess(p.Pid); err == nil {
				return errors.Errorf("pid %d is still running", p.Pid)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for lorgnette to exit")
		}
	}
	return nil
}

// ListScanners calls lorgnette's ListScanners() method and returns the
// (possibly empty) list of ScannerInfo objects from the response.
func (l *Lorgnette) ListScanners(ctx context.Context) ([]*lpb.ScannerInfo, error) {
	call := l.obj.CallWithContext(ctx, dbusInterface+".ListScanners", 0)
	if call.Err != nil {
		return nil, errors.Wrap(call.Err, "failed to call ListScanners")
	}

	var marshalled []byte
	call.Store(&marshalled)
	response := &lpb.ListScannersResponse{}
	if err := proto.Unmarshal(marshalled, response); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal ListScannersResponse")
	}

	return response.Scanners, nil
}
