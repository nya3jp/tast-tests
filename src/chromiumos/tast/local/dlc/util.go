// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides ways to interact with dlcservice daemon and utilities.
package dlc

import (
	"context"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
)

// CopyFile copies file |from| to |to| and sets permissions.
func CopyFile(from, to string, perms os.FileMode) error {
	b, err := ioutil.ReadFile(from)
	if err != nil {
		return errors.Wrap(err, "failed to read file")
	}
	if err := ioutil.WriteFile(to, b, perms); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	return nil
}

// ChownContentsToDlcservice recursively changes the ownership of the directory
// contents to the uid and gid of dlcservice.
func ChownContentsToDlcservice(dir string) error {
	var u *user.User
	var err error
	if u, err = user.Lookup(User); err != nil {
		return err
	}

	var uid, gid int64
	if uid, err = strconv.ParseInt(u.Uid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse uid %q", u.Uid)
	}
	if gid, err = strconv.ParseInt(u.Gid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse gid %q", u.Gid)
	}

	return filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, int(uid), int(gid))
	})
}

// RestartUpstartJobAndWait restarts the given job and waits for it to come up.
func RestartUpstartJobAndWait(ctx context.Context, job, serviceName string) error {
	// Restart job.
	if err := upstart.RestartJob(ctx, job); err != nil {
		return errors.Wrapf(err, "failed to restart %s", job)
	}

	// Wait for service to be ready.
	if bus, err := dbusutil.SystemBus(); err != nil {
		return errors.Wrap(err, "failed to connect to the message bus")
	} else if err := dbusutil.WaitForService(ctx, bus, serviceName); err != nil {
		return errors.Wrapf(err, "failed to wait for D-Bus service %s", serviceName)
	}
	return nil
}
