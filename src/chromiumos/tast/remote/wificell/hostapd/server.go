// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/common/network/daemonutil"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	hostapdCmd = "hostapd"
	hostapdCLI = "/usr/bin/hostapd_cli"
)

// KillAll kills all running hostapd on host, useful for environment setup/cleanup.
func KillAll(ctx context.Context, host *ssh.Conn) error {
	return host.Command("killall", hostapdCmd).Run(ctx)
}

// Server controls a hostapd on router.
type Server struct {
	host    *ssh.Conn
	name    string
	iface   string
	workDir string
	conf    *Config

	cmd        *ssh.Cmd
	wg         sync.WaitGroup
	stdoutFile *os.File
	stderrFile *os.File
}

// StartServer creates a new Server object and runs hostapd on iface of the given host with settings
// specified in config. workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
// After getting a Server instance, s, the caller should call s.Close() at the end, and use the
// shortened ctx (provided by s.ReserveForClose()) before s.Close() to reserve time for it to run.
func StartServer(ctx context.Context, host *ssh.Conn, name, iface, workDir string, config *Config) (server *Server, retErr error) {
	ctx, st := timing.Start(ctx, "hostapd.StartServer")
	defer st.End()

	// Copying the struct config, because hostapd is keeing a *Config pointer from the caller.
	// That could cause a problem if the caller assuems it's read-only.
	hostapdConfigCopy := *config

	s := &Server{
		host:    host,
		name:    name,
		iface:   iface,
		workDir: workDir,
		conf:    &hostapdConfigCopy,
	}
	// Clean up on error.
	defer func() {
		if retErr != nil {
			// Close the Server instance created above, not the returned one as it might be nil.
			s.Close(ctx)
		}
	}()

	if err := s.initConfig(ctx); err != nil {
		return nil, err
	}
	if err := s.start(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// filename returns a filename for s to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (s *Server) filename(suffix string) string {
	return fmt.Sprintf("hostapd-%s-%s.%s", s.name, s.iface, suffix)
}

// confPath returns the path of s's config file.
func (s *Server) confPath() string {
	return path.Join(s.workDir, s.filename("conf"))
}

// ctrlPath returns the path of s's control socket.
func (s *Server) ctrlPath() string {
	return path.Join(s.workDir, s.filename("ctrl"))
}

// stdoutFilename returns the filename under OutDir to store stdout of this hostapd.
func (s *Server) stdoutFilename() string {
	return s.filename("stdout")
}

// stderrFilename returns the filename under OutDir to store stderr of this hostapd.
func (s *Server) stderrFilename() string {
	return s.filename("stderr")
}

// initConfig writes a hostapd config file.
func (s *Server) initConfig(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "initConfig")
	defer st.End()

	conf, err := s.conf.Format(s.iface, s.ctrlPath())
	if err != nil {
		return err
	}
	if err := linuxssh.WriteFile(ctx, s.host, s.confPath(), []byte(conf), 0644); err != nil {
		return errors.Wrap(err, "failed to write config")
	}
	return nil
}

// start spawns a hostapd daemon and waits until it is ready.
func (s *Server) start(fullCtx context.Context) (retErr error) {
	fullCtx, st := timing.Start(fullCtx, "start")
	defer st.End()

	defer func() {
		if retErr != nil {
			s.Close(fullCtx)
		}
	}()

	ctx, cancel := s.ReserveForClose(fullCtx)
	defer cancel()

	testing.ContextLogf(ctx, "Starting hostapd %s on interface %s", s.name, s.iface)
	// TODO(crbug.com/1047146): Remove the env part after we drop the old crypto like MD5.
	cmdStrs := []string{
		// Environment variables.
		"OPENSSL_CONF=/etc/ssl/openssl.cnf.compat",
		"OPENSSL_CHROMIUM_SKIP_TRUSTED_PURPOSE_CHECK=1",
		// hostapd command.
		hostapdCmd, "-dd", "-t", "-K", shutil.Escape(s.confPath()),
	}
	cmd := s.host.Command("sh", "-c", strings.Join(cmdStrs, " "))
	// Prepare stdout/stderr log files.
	var err error
	s.stderrFile, err = fileutil.PrepareOutDirFile(ctx, s.stderrFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of hostapd")
	}
	cmd.Stderr = s.stderrFile

	s.stdoutFile, err = fileutil.PrepareOutDirFile(ctx, s.stdoutFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of hostapd")
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StdoutPipe of hostapd")
	}
	readyFunc := func(buf []byte) (bool, error) {
		if bytes.Contains(buf, []byte("Interface initialization failed")) {
			return false, errors.New("hostapd failed to initialize AP interface")
		} else if bytes.Contains(buf, []byte("Setup of interface done")) {
			return true, nil
		}
		return false, nil
	}

	// Wait for hostapd to get ready.
	readyWriter := daemonutil.NewReadyWriter(readyFunc)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer stdoutPipe.Close()
		defer readyWriter.Close()
		multiWriter := io.MultiWriter(s.stdoutFile, readyWriter)
		io.Copy(multiWriter, stdoutPipe)
	}()

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	s.cmd = cmd

	// Wait for hostapd to get ready.
	if err := readyWriter.Wait(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before s.Close() to reserve time for it to run.
func (s *Server) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 2*time.Second)
}

// Close stops hostapd and cleans up related resources.
func (s *Server) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "hostapd.Close")
	defer st.End()

	testing.ContextLog(ctx, "Stopping hostapd")
	if s.cmd != nil {
		s.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		s.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", hostapdCmd, s.confPath())).Run(ctx)

		// Skip the error in Wait as the process is aborted and always has error in wait.
		s.cmd.Wait(ctx)
		s.cmd = nil
	}
	// Wait the bg routine to end before closing files.
	s.wg.Wait()
	if s.stdoutFile != nil {
		s.stdoutFile.Close()
	}
	if s.stderrFile != nil {
		s.stderrFile.Close()
	}
	if err := s.host.Command("rm", s.confPath()).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to remove config")
	}
	return nil
}

// hostapdCLI is a helper function for running hostapd_cli command to control
// this Server.
func (s *Server) hostapdCLI(ctx context.Context, args ...string) (string, error) {
	fullArgs := append([]string{
		"-p" + s.ctrlPath(),
		"-i" + s.Interface(),
	}, args...)
	raw, err := s.host.Command(hostapdCLI, fullArgs...).Output(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run hostapd_cli, stdout=%q", string(raw))
	}
	return string(raw), nil
}

// DeauthClient deauthenticates the client with specified MAC address.
func (s *Server) DeauthClient(ctx context.Context, clientMAC string) error {
	if err := s.host.Command(hostapdCLI, fmt.Sprintf("-p%s", s.ctrlPath()), "deauthenticate", clientMAC).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to deauthenticate client with MAC address %s", clientMAC)
	}

	return nil
}

// BSSTMReqParams defines the parameters for a BSS Transition Management Request.
type BSSTMReqParams struct {
	// Neighbors is the list of neighboring APs
	Neighbors []string
	// DisassocImminent indicates whether or not the AP will disassociate the STA soon.
	DisassocImminent bool
	// DisassocTimer is the time (in 100ms) before the AP will disassoc the STA.
	DisassocTimer time.Duration
	// ReassocDelay is the delay (in seconds) before the STA is permitted to reassociate to the AP.
	ReassocDelay time.Duration
	// BSSTerm is the time before the AP will be terminated
	BSSTerm time.Duration
}

// SendBSSTMRequest sends a BSS Transition Management Request to the specified client.
func (s *Server) SendBSSTMRequest(ctx context.Context, clientMAC string, params BSSTMReqParams) error {
	// Construct the arguments for:
	//   `hostapd_cli -p${ctrlPath} BSS_TM_REQ ${clientMAC} neighbor=${n},0,0,0,0 pref=1`
	args := []string{"BSS_TM_REQ", clientMAC}
	for _, n := range params.Neighbors {
		args = append(args, fmt.Sprintf("neighbor=%s,0,0,0,0", n))
	}
	args = append(args, "pref=1")
	if params.DisassocImminent {
		args = append(args, "disassoc_imminent=1")
		args = append(args, fmt.Sprintf("disassoc_timer=%d", (params.DisassocTimer/(100*time.Millisecond))))
		args = append(args, fmt.Sprintf("mbo=3:%d:0", params.ReassocDelay/time.Second))
	}
	if params.BSSTerm > 0 {
		args = append(args, fmt.Sprintf("bss_term=0,%d", params.BSSTerm/time.Minute))
	}

	// Run the command
	if _, err := s.hostapdCLI(ctx, args...); err != nil {
		return errors.Wrapf(err, "failed to send BSS TM request to client %s", clientMAC)
	}
	return nil
}

// Property is the property name of a hostapd property
type Property string

const (
	// PropertyMBOAssocDisallow prevents association to hostapd if set to 1.
	PropertyMBOAssocDisallow Property = "mbo_assoc_disallow"
)

// Set sets a hostapd property prop to value val
func (s *Server) Set(ctx context.Context, prop Property, val string) error {
	args := []string{"SET", string(prop), val}
	if _, err := s.hostapdCLI(ctx, args...); err != nil {
		return errors.Wrapf(err, "failed to set property %v to value %v", prop, val)
	}
	return nil
}

// Interface returns the interface used by the hostapd.
func (s *Server) Interface() string {
	return s.iface
}

// Name returns the name used by the hostapd.
func (s *Server) Name() string {
	return s.name
}

// Config returns the config used by the hostapd.
// NOTE: Caller should not modify the returned object.
func (s *Server) Config() *Config {
	return s.conf
}

// CSOption is the function signature used to specify options of CSA command.
type CSOption func(*csaConfig)

// CSAMode returns an Option which sets mode in CSA.
func CSAMode(m string) CSOption {
	return func(c *csaConfig) {
		c.mode = m
	}
}

// csaConfig is the configuration for the channel switch announcement.
type csaConfig struct {
	mode string
}

// StartChannelSwitch initiates a channel switch in the AP.
func (s *Server) StartChannelSwitch(ctx context.Context, csCount, csChannel int, options ...CSOption) error {
	csFreq, err := ChannelToFrequency(csChannel)
	if err != nil {
		return errors.Wrap(err, "failed to convert channel to frequency")
	}
	cfg := &csaConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	var args []string
	args = append(args, "chan_switch")
	args = append(args, strconv.Itoa(csCount))
	args = append(args, strconv.Itoa(csFreq))
	if cfg.mode != "" {
		args = append(args, cfg.mode)
	}

	if _, err := s.hostapdCLI(ctx, args...); err != nil {
		return errors.Wrapf(err, "failed to send CSA with freq %d", csFreq)
	}

	// Wait for the AP to change channel.
	iwr := iw.NewRemoteRunner(s.host)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		chConfig, err := iwr.RadioConfig(ctx, s.iface)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the radio configuration"))
		}
		if chConfig.Number == csChannel {
			// Update hostapd channel.
			s.conf.Channel = csChannel
			return nil
		}
		return errors.Errorf("failed to switch to the alternate channel %d", csChannel)
	}, &testing.PollOptions{
		Timeout:  3 * time.Second,
		Interval: 200 * time.Millisecond,
	}); err != nil {
		return err
	}

	return nil
}

// ListSTA lists the MAC addresses of connected STAs.
func (s *Server) ListSTA(ctx context.Context) ([]string, error) {
	payload, err := s.hostapdCLI(ctx, "list_sta")
	if err != nil {
		return nil, err
	}

	return strings.Fields(payload), nil
}
