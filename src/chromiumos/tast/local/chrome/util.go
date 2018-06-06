// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

const (
	pollInterval = 10 * time.Millisecond
)

// poll runs f repeatedly until it returns nil and then also returns nil.
// If ctx returns an error before then, the last error returned by f is returned.
func poll(ctx context.Context, f func() error) error {
	var last error
	for {
		if last = f(); last == nil {
			return nil
		}
		if ctx.Err() != nil {
			return last
		}
		time.Sleep(pollInterval)

		// f may be using ctx as well, which could lead to it producing a less-useful
		// error during the next iteration if it also checks if the deadline was reached.
		// Check if the context expired while we were sleeping and return the last error
		// that f returned if so.
		if ctx.Err() != nil {
			return last
		}
	}
}

// chownContents recursively chowns dir's contents to username's uid and gid.
func chownContents(dir string, username string) error {
	var u *user.User
	var err error
	if u, err = user.Lookup(username); err != nil {
		return err
	}

	var uid, gid int64
	if uid, err = strconv.ParseInt(u.Uid, 10, 32); err != nil {
		return fmt.Errorf("failed to parse uid %q: %v", u.Uid, err)
	}
	if gid, err = strconv.ParseInt(u.Gid, 10, 32); err != nil {
		return fmt.Errorf("failed to parse gid %q: %v", u.Gid, err)
	}

	return filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, int(uid), int(gid))
	})
}
