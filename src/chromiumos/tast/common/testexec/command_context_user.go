// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testexec

import (
	"context"
	"os/user"
	"strconv"
	"syscall"

	"chromiumos/tast/errors"
)

// CommandContextUser creates a CommandContext that will run as the
// given username.
//
// The returned error is a result of the user/group lookup steps.
func CommandContextUser(ctx context.Context, username, name string, arg ...string) (*Cmd, error) {
	usr, err := user.Lookup(username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to lookup username")
	}
	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse uid")
	}
	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse gid")
	}
	grps, err := usr.GroupIds()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve groups")
	}
	gids := make([]uint32, len(grps))
	for i, g := range grps {
		id, err := strconv.Atoi(g)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse groups id")
		}
		gids[i] = uint32(id)
	}

	cmd := CommandContext(ctx, name, arg...)
	if cmd.Cmd.SysProcAttr == nil {
		cmd.Cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// We must provide all fields or they will not be populated when the
	// process starts.
	cmd.Cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid:    uint32(uid),
		Gid:    uint32(gid),
		Groups: gids,
	}
	return cmd, nil
}
