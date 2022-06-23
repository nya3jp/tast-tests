// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tcpdump

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Option is the type of options to start Capturer object.
type Option func(*Capturer)

// Snaplen returns an option which sets a Capturer's snapshot length.
func Snaplen(s uint64) Option {
	return func(c *Capturer) {
		c.snaplen = s
	}
}

// Capturer controls a tcpdump process to capture packets on an interface.
type Capturer struct {
	name    string
	iface   string
	workDir string

	snaplen uint64

	wg         sync.WaitGroup
	cmd        *testexec.Cmd
	stdoutFile *os.File
	stderrFile *os.File
}

const (
	tcpdumpCmd               = "tcpdump"
	durationForClose         = 4 * time.Second
	durationForInternalClose = 2 * time.Second
)

// StartCapturer creates and starts a Capturer.
// After getting a Capturer instance, c, the caller should call c.Close() at the end, and use the
// shortened ctx (provided by c.ReserveForClose()) before c.Close() to reserve time for it to run.
func StartCapturer(ctx context.Context, name, iface, workDir string, opts ...Option) (*Capturer, error) {
	c := &Capturer{
		name:    name,
		iface:   iface,
		workDir: workDir,
	}
	for _, opt := range opts {
		opt(c)
	}

	if err := c.start(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Capturer) start(fullCtx context.Context) (err error) {
	// Clean up on error.
	defer func() {
		if err != nil {
			c.close(fullCtx)
		}
	}()

	// Reserve time for the above deferred call.
	ctx, ctxCancel := ctxutil.Shorten(fullCtx, durationForInternalClose)
	defer ctxCancel()

	testing.ContextLogf(ctx, "Starting capturer on %s", c.iface)

	args := []string{"-U", "-i", c.iface, "-w", c.packetPath()}
	if c.snaplen != 0 {
		args = append(args, "-s", strconv.FormatUint(c.snaplen, 10))
	}

	cmd := testexec.CommandContext(ctx, tcpdumpCmd, args...)
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}

	c.cmd = cmd
	testing.ContextLog(ctx, "Waiting 5 seconds for tcpdump to be ready")
	testing.Sleep(ctx, 5*time.Second)

	return nil
}

// close kills the process, tries to releases occupied resources.
func (c *Capturer) close(ctx context.Context) error {
	var err error
	if c.cmd != nil {
		// Kill with SIGTERM here, so that the process can flush buffer.
		// If the process does not die before deadline, cmd.Wait will then abort it.
		// TODO(crbug.com/1030635): Signal through SSH might not work. Use pkill to send signal for now.
		testexec.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("^%s.*%s", tcpdumpCmd, c.packetPath())).Run()
		c.cmd.Wait()
	}
	return err
}

// OutputTCP will output all records related to the tcp protocol, and delete the file after output.
func (c *Capturer) OutputTCP(ctx context.Context) ([]byte, error) {
	out, err := testexec.CommandContext(ctx, "tcpdump", "tcp", "-r", c.packetPath()).Output()
	if err != nil {
		return nil, err
	}
	if err := testexec.CommandContext(ctx, "rm", c.packetPath()).Run(); err != nil {
		return nil, errors.Wrapf(err, "failed to clean up tcpdump file %s", c.packetPath())
	}
	return out, nil
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before c.Close() to reserve time for it to run.
func (c *Capturer) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, durationForClose)
}

// Close terminates the capturer and downloads the pcap file from host to OutDir.
func (c *Capturer) Close(ctx context.Context) error {
	// Wait 2 seconds (2 * libpcap poll timeout) before killing the
	// process so that it can properly catch all packets.
	// Investigation of the timeout can be found in crrev.com/c/288814.
	// TODO(b/154787243): Find a better way to wait for pcap to be done.
	testing.Sleep(ctx, 2*time.Second)
	return c.close(ctx)
}

// packetPath returns the path for tcpdump to write parsed packets.
func (c *Capturer) packetPath() string {
	return filepath.Join(c.workDir, c.filename("pcap"))
}

// filename returns a filename for c to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (c *Capturer) filename(suffix string) string {
	return fmt.Sprintf("pcap-%s.%s", c.name, suffix)
}
