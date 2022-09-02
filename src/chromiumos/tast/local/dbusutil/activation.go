// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
)

const (
	statefulPartitionDir = "/mnt/stateful_partition"
	dbusServiceDir       = "/usr/share/dbus-1/system-services"
)

// IsDbusActivationEnabled returns true if given service can be
// automatically started.
func IsDbusActivationEnabled(service string) (bool, error) {
	serviceFile := service + ".service"
	servicePath := filepath.Join(dbusServiceDir, serviceFile)
	if _, err := os.Stat(servicePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// EnableDbusActivation enables mechanism that will automatically start
// given service on connection attempt. The rootfs must be writable when this
// function is called.
func EnableDbusActivation(ctx context.Context, service string) error {
	return changeDbusActivationStatus(ctx, service, true)
}

// DisableDbusActivation disables mechanism that would automatically start
// given service on connection attempt. The rootfs must be writable when this
// function is called.
func DisableDbusActivation(ctx context.Context, service string) error {
	return changeDbusActivationStatus(ctx, service, false)
}

// changeDbusActivationStatus is a util function used to control DBus activation
// state for given service.
func changeDbusActivationStatus(ctx context.Context, service string, enable bool) error {
	var dst, src string

	serviceFile := service + ".service"
	servicePath := filepath.Join(dbusServiceDir, serviceFile)
	statefulServicePath := filepath.Join(statefulPartitionDir, serviceFile)
	if enable {
		src = statefulServicePath
		dst = servicePath
	} else {
		src = servicePath
		dst = statefulServicePath
	}
	if err := fsutil.MoveFile(src, dst); err != nil {
		return errors.Wrapf(err, "failed to move file from %q to %q", src, dst)
	}
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to sync DUT")
	}
	return nil

}
