// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package framesender provides utilities to send management frames.
package framesender

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const cmdSendManagementFrame = "send_management_frame"

// Sender sends management frame with send_management_frame tool provided by
// wifi-testbed package on test router.
type Sender struct {
	host    *ssh.Conn
	iface   string
	workDir string
	curCmd  *ssh.CmdCtx
	logFile *os.File
}

// Type is the enum type of frame type.
type Type string

// Type enum values.
const (
	TypeBeacon        Type = "beacon"
	TypeChannelSwitch Type = "channel_switch"
	TypeProbeResponse Type = "probe_response"
)

// config contains the configuration of Sender.Send call.
type config struct {
	t          Type
	ch         int
	count      int
	ssidPrefix string
	numBSS     int
	delay      int
	destMAC    string
	footer     []byte
}

// Option is the type of options of Sender.Send call.
type Option func(*config)

// Count returns an Option which sets the count to send in Send config.
// 0 is a special value meaning endless send. When count=0 specified,
// the process will only stop on context done.
func Count(count int) Option {
	return func(c *config) {
		c.count = count
	}
}

// SSIDPrefix returns an Option which sets the SSID prefix.
func SSIDPrefix(p string) Option {
	return func(c *config) {
		c.ssidPrefix = p
	}
}

// NumBSS returns an Option which sets the number of BSS.
func NumBSS(n int) Option {
	return func(c *config) {
		c.numBSS = n
	}
}

// Delay returns an Option which sets the delay (in milliseconds) between frames.
func Delay(d int) Option {
	return func(c *config) {
		c.delay = d
	}
}

// DestMAC returns an Option which sets the destination MAC.
func DestMAC(mac string) Option {
	return func(c *config) {
		c.destMAC = mac
	}
}

// ProbeRespFooter returns an Option which sets the footer in probe response.
func ProbeRespFooter(b []byte) Option {
	return func(c *config) {
		c.footer = b
	}
}

// New creates a Sender object on host.
func New(host *ssh.Conn, iface, workDir string) *Sender {
	return &Sender{
		host:    host,
		iface:   iface,
		workDir: workDir,
	}
}

// Send executes send_management_frame tool to send management frames with type t
// on iface and ch channel with given options.
func (s *Sender) Send(ctx context.Context, t Type, ch int, ops ...Option) error {
	if err := s.Start(ctx, t, ch, ops...); err != nil {
		return err
	}
	return s.wait()
}

// Start runs send_management_frame tool in background to send management
// frames with type t on iface and ch channel with given options.
func (s *Sender) Start(ctx context.Context, t Type, ch int, ops ...Option) error {
	if s.curCmd != nil {
		return errors.New("already running")
	}
	c := &config{
		t:     t,
		ch:    ch,
		count: 1,
	}
	for _, op := range ops {
		op(c)
	}
	args, err := s.configToArgs(ctx, c)
	if err != nil {
		return err
	}

	// Prepare the log file under OutDir.
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get OutDir")
	}
	f, err := ioutil.TempFile(outdir, "frame_sender_*.log")
	if err != nil {
		return errors.Wrap(err, "failed to create log file for frame_sender")
	}
	testing.ContextLogf(ctx, "Logging send_management_frame output to %q", filepath.Base(f.Name()))

	cmd := s.host.CommandContext(ctx, cmdSendManagementFrame, args...)
	// Collect combined output to f.
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		f.Close()
		return errors.Wrap(err, "failed to start send_management_frame")
	}
	s.curCmd = cmd
	s.logFile = f
	return nil
}

// wait waits the current running command to end and free related resources.
func (s *Sender) wait() error {
	err := s.curCmd.Wait()
	s.logFile.Close()
	s.curCmd = nil
	s.logFile = nil
	return err
}

// ReserveForStop reserves time in context for cleaning up in Stop.
func (s Sender) ReserveForStop(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 3*time.Second)
}

// Stop aborts current running command spawned by previous Start call.
func (s *Sender) Stop(ctx context.Context) error {
	if s.curCmd == nil {
		return errors.New("no running command")
	}
	// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
	s.host.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("^%s.*%s", cmdSendManagementFrame, s.Interface())).Run()
	s.curCmd.Abort()
	// Ignore the error from wait, as aborted cmd will always fail.
	s.wait()
	return nil
}

// configToArgs generates the command line arguments for config.
func (s *Sender) configToArgs(ctx context.Context, c *config) ([]string, error) {
	args := []string{
		"-i", s.iface,
		"-t", string(c.t),
		"-c", strconv.Itoa(c.ch),
		"-n", strconv.Itoa(c.count),
	}
	if c.ssidPrefix != "" {
		args = append(args, "-s", c.ssidPrefix)
	}
	if c.numBSS != 0 {
		args = append(args, "-b", strconv.Itoa(c.numBSS))
	}
	if c.delay != 0 {
		args = append(args, "-d", strconv.Itoa(c.delay))
	}
	if c.destMAC != "" {
		args = append(args, "-a", c.destMAC)
	}
	if len(c.footer) != 0 {
		pattern := path.Join(s.workDir, "probe_response_footer_XXX")
		footerPath, err := fileutil.WriteTmp(ctx, s.host, pattern, c.footer)
		if err != nil {
			return nil, errors.Wrap(err, "failed to prepare footer file")
		}
		args = append(args, "-f", footerPath)
	}
	return args, nil
}

// Interface returns the interface that this sender works on.
func (s *Sender) Interface() string {
	return s.iface
}
