// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pcap

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/testing"
)

// Option is the type of options to start Capturer object.
type Option func(*Capturer)

// Snaplen returns an option which sets snaplen of Capturer.
func Snaplen(s uint64) Option {
	return func(c *Capturer) {
		c.snaplen = s
	}
}

// Capturer controls a tcpdump process to capture packets on an interface.
type Capturer struct {
	host    *host.SSH
	name    string
	iface   string
	workDir string

	snaplen uint64

	cmd        *host.Cmd
	stdoutFile *os.File
	stderrFile *os.File
}

const tcpdumpCmd = "tcpdump"

// StartCapturer creates and starts a Capturer.
func StartCapturer(ctx context.Context, host *host.SSH, name, iface, workDir string, opts ...Option) (*Capturer, error) {
	c := &Capturer{
		host:    host,
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

func (c *Capturer) start(ctx context.Context) (err error) {
	// Clean up on error.
	defer func() {
		if err != nil {
			c.Close(ctx)
		}
	}()

	args := []string{"-i", c.iface, "-w", c.packetPathOnHost()}
	if c.snaplen != 0 {
		args = append(args, "-s", strconv.FormatUint(c.snaplen, 10))
	}

	cmd := c.host.Command(tcpdumpCmd, args...)
	c.stdoutFile, err = fileutil.PrepareOutDirFile(ctx, c.filename("stdout"))
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of tcpdump")
	}
	c.stderrFile, err = fileutil.PrepareOutDirFile(ctx, c.filename("stderr"))
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of tcpdump")
	}
	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}
	c.cmd = cmd
	return nil
}

// Close terminates the capturer and downloads the pcap file from host to workstation.
func (c *Capturer) Close(ctx context.Context) error {
	var err error
	if c.cmd != nil {
		// Kill with SIGTERM here, so that the process can flush buffer.
		// If the process does not die before deadline, cmd.Wait will then abort it.
		// TODO(crbug.com/1030635): Signal through SSH might not work. Use pkill to send signal for now.
		c.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", tcpdumpCmd, c.packetPathOnHost())).Run(ctx)
		c.cmd.Wait(ctx)
		err = c.downloadPacket(ctx)
	}
	if c.stderrFile != nil {
		c.stdoutFile.Close()
	}
	if c.stdoutFile != nil {
		c.stderrFile.Close()
	}
	return err
}

// Interface returns the interface that the capturer runs on.
func (c *Capturer) Interface() string {
	return c.iface
}

// filename returns a filename for c to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (c *Capturer) filename(suffix string) string {
	return fmt.Sprintf("pcap-%s.%s", c.name, suffix)
}

// packetPathOnHost returns the temporary path on c.host for tcpdump to write parsed packets.
func (c *Capturer) packetPathOnHost() string {
	return path.Join(c.workDir, c.filename("pcap.tmp"))
}

// packetFilename returns the path under OutDir that capturer save the pcap file on Close call.
func (c *Capturer) packetFilename() string {
	return c.filename("pcap")
}

// downloadPacket downloads result pcap file from host to workstation.
func (c *Capturer) downloadPacket(ctx context.Context) error {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get OutDir")
	}
	src := c.packetPathOnHost()
	dst := path.Join(outDir, c.packetFilename())
	if err := c.host.GetFile(ctx, src, dst); err != nil {
		return errors.Wrapf(err, "unable to download packet from %s to %s", src, dst)
	}
	return nil
}
