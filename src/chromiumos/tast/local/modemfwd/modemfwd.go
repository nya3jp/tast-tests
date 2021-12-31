// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package modemfwd interacts with modemfwd D-Bus service.
package modemfwd

import (
	"context"
	"io"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	dbusPath      = "/org/chromium/Modemfwd"
	dbusName      = "org.chromium.Modemfwd"
	dbusInterface = "org.chromium.Modemfwd"
	// JobName is the name of the modemfwd process
	JobName = "modemfwd"
	// DisableAutoUpdatePref disables auto update on modemfwd
	DisableAutoUpdatePref = "/var/lib/modemfwd/disable_auto_update"
)

// Modemfwd is used to interact with the modemfwd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/modemfwd/dbus_bindings/org.chromium.Modemfwd.xml
type Modemfwd struct {
	*dbusutil.DBusObject
}

// New connects to modemfwd via D-Bus and returns a Modemfwd object.
func New(ctx context.Context) (*Modemfwd, error) {
	obj, err := dbusutil.NewDBusObject(ctx, dbusName, dbusInterface, dbusPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to modemfwd")
	}
	return &Modemfwd{obj}, nil
}

// ForceFlash calls modemfwd's ForceFlash D-Bus method.
func (m *Modemfwd) ForceFlash(ctx context.Context, device string, options map[string]string) error {
	result := false
	if err := m.Call(ctx, "ForceFlash", device, options).Store(&result); err != nil {
		return err
	}
	if !result {
		return errors.New("ForceFlash returned false")
	}
	return nil
}

// StartAndWaitForQuiescence starts the modemfwd job and waits for the initial sequence to complete
// or until an error is logged.
func StartAndWaitForQuiescence(ctx context.Context) error {
	reader, err := syslog.NewReader(ctx, syslog.Program(JobName))
	if err != nil {
		return errors.Wrap(err, "failed to create log reader")
	}
	// Ensure the Reader is closed.
	defer reader.Close()

	err = upstart.StartJob(ctx, JobName)
	if err != nil {
		return errors.Wrapf(err, "failed to start %q", JobName)
	}

	// Wait for an error/success log message from modemfwd.
	// If the message is never logged, the test will fail due to a timeout.
	for {
		e, err := reader.Read()
		if err == io.EOF {
			testing.Sleep(ctx, 500*time.Millisecond)
			continue
		}
		if err != nil {
			return errors.Wrap(err, "failed to read syslog")
		}
		if e.Severity == string(syslog.Err) {
			return errors.New("modemfwd reported the error: " + e.Content)
		}
		if strings.Contains(e.Content, "The modem already has the correct firmware installed") ||
			strings.Contains(e.Content, "Update disabled by pref") {
			break
		}
	}
	return nil
}
