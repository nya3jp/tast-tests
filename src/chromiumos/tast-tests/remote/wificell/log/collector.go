// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package log

import (
	"bytes"
	"context"
	"io"
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// buffer is the buffer object for collected logs.
type buffer struct {
	lock sync.Mutex
	buf  bytes.Buffer
}

// Write writes d into the buffer.
func (b *buffer) Write(d []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buf.Write(d)
}

// Dump copies the buffer to w and resets the buffer.
func (b *buffer) Dump(w io.Writer) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, err := b.buf.WriteTo(w); err != nil {
		return err
	}
	b.buf.Reset()
	return nil
}

// Collector watches a file on remote host and collects the appended contents.
type Collector struct {
	host                    *ssh.Conn
	buf                     buffer
	path                    string
	cmd                     *ssh.Cmd
	tailFollowNameSupported bool
}

// StartCollector spawns a log collector on file p on host.
//
// Set tailFollowNameSupported to true if the host's tail implementation
// supports the "--follow=name" command to allow for tail following to stay
// at the same filename rather than follow the file if the name changes.
func StartCollector(ctx context.Context, host *ssh.Conn, p string, tailFollowNameSupported bool) (*Collector, error) {
	c := &Collector{
		host:                    host,
		path:                    p,
		tailFollowNameSupported: tailFollowNameSupported,
	}
	if err := c.start(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// start spawns the tail command to track the target log file.
func (c *Collector) start(ctx context.Context) error {
	var followArg string
	if c.tailFollowNameSupported {
		followArg = "--follow=name"
	} else {
		followArg = "-f"
	}

	cmd := c.host.CommandContext(ctx, "tail", followArg, c.path)

	cmd.Stdout = &c.buf

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to run tail command")
	}
	c.cmd = cmd
	return nil
}

// Dump copies the contents collected to w and resets the buffer.
func (c *Collector) Dump(w io.Writer) error {
	return c.buf.Dump(w)
}

// Close stops the collector.
func (c *Collector) Close() error {
	c.cmd.Abort()
	// Ignore the error as wait always has error on aborted command.
	_ = c.cmd.Wait()
	return nil
}
