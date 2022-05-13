// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strings"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// NewSignalWatcher creates an D-Bus signal watcher on PowerManager interface.
func NewSignalWatcher(ctx context.Context, signalNames ...string) (*dbusutil.SignalWatcher, error) {
	if len(signalNames) == 0 {
		return nil, errors.New("no signal to watch")
	}
	var matches []dbusutil.MatchSpec
	for _, signalName := range signalNames {
		matches = append(matches, dbusutil.MatchSpec{
			Type:      "signal",
			Path:      dbus.ObjectPath(dbusPath),
			Interface: dbusInterface,
			Member:    signalName,
		})
	}
	watcher, err := dbusutil.NewSignalWatcherForSystemBus(ctx, matches...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create dbus watcher")
	}
	return watcher, nil
}

// SignalName extracts the signal name from a dbus.Signal object.
func SignalName(s *dbus.Signal) string {
	parts := strings.Split(s.Name, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
