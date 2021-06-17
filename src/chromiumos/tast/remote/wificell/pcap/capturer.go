// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pcap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/common/network/daemonutil"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
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
	host    *ssh.Conn
	name    string
	iface   string
	workDir string

	snaplen uint64

	cmd        *ssh.Cmd
	stdoutFile *os.File
	stderrFile *os.File
	wg         sync.WaitGroup
	downloaded bool
}

const (
	tcpdumpCmd               = "tcpdump"
	durationForClose         = 4 * time.Second
	durationForInternalClose = 2 * time.Second
)

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

	args := []string{"-U", "-i", c.iface, "-w", c.packetPathOnRemote()}
	if c.snaplen != 0 {
		args = append(args, "-s", strconv.FormatUint(c.snaplen, 10))
	}

	cmd := c.host.Command(tcpdumpCmd, args...)
	c.stdoutFile, err = fileutil.PrepareOutDirFile(ctx, c.filename("stdout"))
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of tcpdump")
	}
	cmd.Stdout = c.stdoutFile

	c.stderrFile, err = fileutil.PrepareOutDirFile(ctx, c.filename("stderr"))
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of tcpdump")
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StderrPipe of tcpdump")
	}
	readyFunc := func(buf []byte) (bool, error) {
		return bytes.Contains(buf, []byte("listening on")), nil
	}
	readyWriter := daemonutil.NewReadyWriter(readyFunc)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer stderrPipe.Close()
		defer readyWriter.Close()
		multiWriter := io.MultiWriter(c.stderrFile, readyWriter)
		io.Copy(multiWriter, stderrPipe)
	}()

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}
	c.cmd = cmd

	testing.ContextLog(ctx, "Waiting for tcpdump to be ready")
	readyCtx, readyCtxCancel := context.WithTimeout(ctx, 15*time.Second)
	defer readyCtxCancel()
	if err := readyWriter.Wait(readyCtx); err != nil {
		return err
	}

	return nil
}

// close kills the process, tries to download the packet file if available and
// releases occupied resources.
func (c *Capturer) close(ctx context.Context) error {
	var err error
	if c.cmd != nil {
		// Kill with SIGTERM here, so that the process can flush buffer.
		// If the process does not die before deadline, cmd.Wait will then abort it.
		// TODO(crbug.com/1030635): Signal through SSH might not work. Use pkill to send signal for now.
		c.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", tcpdumpCmd, c.packetPathOnRemote())).Run(ctx)
		c.cmd.Wait(ctx)
		err = c.downloadPacket(ctx)
	}
	// Wait for the bg routine to end before closing files.
	c.wg.Wait()
	if c.stderrFile != nil {
		c.stderrFile.Close()
	}
	if c.stdoutFile != nil {
		c.stdoutFile.Close()
	}
	return err
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
	if err := linuxssh.GetFile(ctx, c.host, src, dst, linuxssh.PreserveSymlinks); err != nil {
		return errors.Wrapf(err, "unable to download packet from %s to %s", src, dst)
	}
	c.downloaded = true
	return nil
}
