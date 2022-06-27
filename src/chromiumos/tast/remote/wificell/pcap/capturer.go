// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pcap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"chromiumos/tast/common/network/tcpdump"
	"chromiumos/tast/common/wificell/router"
	"chromiumos/tast/errors"
	remotetcpdump "chromiumos/tast/remote/network/tcpdump"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/ssh"
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
	host       *ssh.Conn
	name       string
	iface      string
	workDir    string
	snaplen    uint64
	cmd        *ssh.Cmd
	stdoutFile *os.File
	stderrFile *os.File
	wg         sync.WaitGroup
	downloaded bool
	runner     *tcpdump.Runner
}

// StartCapturer creates and starts a Capturer.
// After getting a Capturer instance, c, the caller should call c.Close() at the end, and use the
// shortened ctx (provided by c.ReserveForClose()) before c.Close() to reserve time for it to run.
func StartCapturer(ctx context.Context, host *ssh.Conn, name, iface, workDir string, opts ...Option) (*Capturer, error) {
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
	c.stdoutFile, err = fileutil.PrepareOutDirFile(ctx, c.filename("stdout"))
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of tcpdump")
	}
	c.stderrFile, err = fileutil.PrepareOutDirFile(ctx, c.filename("stderr"))
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of tcpdump")
	}
	c.runner = remotetcpdump.NewRemoteRunner(c.host)
	if c.snaplen != 0 {
		c.runner.Snaplen(c.snaplen)
	}
	_, err = c.runner.StartTcpdump(ctx, c.iface, c.packetPathOnRemote(), c.stdoutFile, c.stderrFile)

	if err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}

	return nil
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before c.Close() to reserve time for it to run.
func (c *Capturer) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return c.runner.ReserveForClose(ctx)
}

// Close terminates the capturer and downloads the pcap file from host to OutDir.
func (c *Capturer) Close(ctx context.Context) error {
	c.runner.Close(ctx)
	if c.runner.CmdExists() {
		return c.downloadPacket(ctx)
	}
	return nil
}

// Interface returns the interface the capturer runs on.
func (c *Capturer) Interface() string {
	return c.iface
}

// filename returns a filename for c to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (c *Capturer) filename(suffix string) string {
	return fmt.Sprintf("pcap-%s.%s", c.name, suffix)
}

// packetPathOnRemote returns the temporary path on c.host for tcpdump to write parsed packets.
func (c *Capturer) packetPathOnRemote() string {
	return filepath.Join(c.workDir, c.filename("pcap.tmp"))
}

// packetFilename returns the path under OutDir that capturer save the pcap file on Close call.
func (c *Capturer) packetFilename() string {
	return c.filename("pcap")
}

// packetPath returns the path of the result pcap file.
func (c *Capturer) packetPath(ctx context.Context) (string, error) {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return "", errors.New("failed to get OutDir")
	}
	return filepath.Join(outDir, c.packetFilename()), nil
}

// PacketPath returns the path of the result pcap file so that the tests can
// verify the content of captured packets. This function should be called
// after Close (i.e. packet downloaded), otherwise it will return error.
func (c *Capturer) PacketPath(ctx context.Context) (string, error) {
	if !c.downloaded {
		return "", errors.New("pcap not yet downloaded")
	}
	return c.packetPath(ctx)
}

// downloadPacket downloads result pcap file from host to OutDir.
func (c *Capturer) downloadPacket(ctx context.Context) error {
	dst, err := c.packetPath(ctx)
	if err != nil {
		return err
	}
	src := c.packetPathOnRemote()
	if c.downloaded {
		return errors.Errorf("packet already downloaded from %s to %s", src, dst)
	}
	if err := router.GetSingleFile(ctx, c.host, src, dst); err != nil {
		return errors.Wrapf(err, "unable to download packet from %s to %s", src, dst)
	}
	c.downloaded = true
	if err := c.host.CommandContext(ctx, "rm", src).Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to clean up remote file %s", src)
	}
	return nil
}
