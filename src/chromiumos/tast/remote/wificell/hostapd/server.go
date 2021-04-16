// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket/layers"

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

// MeasurementMode is the beacon request measurement mode to be used
type MeasurementMode uint8

const (
	// ModePassive scan on selected channels
	ModePassive MeasurementMode = 0
	// ModeActive scan on selected channels
	ModeActive MeasurementMode = 1
	// ModeTable uses the cached scan results
	ModeTable MeasurementMode = 2
)

// Detail specifies which IEs to include in the report
type Detail uint8

const (
	// DetailNone omits all IEs from report
	DetailNone Detail = 0
	// DetailRequestedOnly includes only the IEs specified in the request subelement in the report
	DetailRequestedOnly Detail = 1
	// DetailAllFields includes all IEs
	DetailAllFields Detail = 2
)

// SubelemID are the element IDs in beacon reports
type SubelemID uint8

// Some commonly used sub-elements in beacon reports
const (
	SubelemSSID           SubelemID = 0
	SubelemInfo           SubelemID = 1
	SubelemDetail         SubelemID = 2
	SubelemRequest        SubelemID = 10
	SubelemChannelReport  SubelemID = 51
	SubelemLastIndication SubelemID = 164
)

// BeaconReqParams defines the parameters for a beacon request
type BeaconReqParams struct {
	// OpClass is the operating class
	OpClass uint8
	// Channel specifies the channel to scan on
	Channel uint8
	// Duration is the measurement time limit
	Duration uint16
	// Mode is the measurement mode to be used
	Mode MeasurementMode
	// BSSID is the BSSID to scan for
	BSSID net.HardwareAddr
	// SSID is the SSID to scan for
	SSID string
	// ReportingDetail specifies which IEs to include in the report
	ReportingDetail Detail
	// ReportChannels specifies which channels to report on
	ReportChannels []uint8
	// Request lists IEs expected in the report
	Request []layers.Dot11InformationElementID
	// LastFrame indicates whether or not we should indicate that the last report frame is the last frame
	LastFrame bool
}

// Serialize serializes the beacon request parameters into a hex string recognizable by hostapd
func (b BeaconReqParams) Serialize() (string, error) {
	var req []byte
	req = append(req, b.OpClass)
	req = append(req, b.Channel)
	req = append(req, 0, 0) // Two bytes for randomization interval
	durationBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(durationBytes, b.Duration)
	req = append(req, durationBytes...)
	if b.Mode > 2 {
		return "", errors.Errorf("invalid measurement mode: %v. Expected 0, 1 or 2", b.Mode)
	}
	req = append(req, byte(b.Mode))
	req = append(req, b.BSSID...)
	if len(b.SSID) > 0 {
		req = append(req, byte(SubelemSSID), byte(len(b.SSID)))
		req = append(req, b.SSID...)
	}
	if b.ReportingDetail > 2 {
		return "", errors.Errorf("invalid reporting detail: %v. Expected 0, 1, or 2", b.ReportingDetail)
	}
	req = append(req, byte(SubelemDetail), 1, byte(b.ReportingDetail))
	if len(b.ReportChannels) > 0 {
		req = append(req, byte(SubelemChannelReport), byte(len(b.ReportChannels)+1), b.OpClass)
		req = append(req, b.ReportChannels...)
	}
	if len(b.Request) > 0 {
		req = append(req, byte(SubelemRequest), byte(len(b.Request)))
		dst := make([]byte, len(b.Request))
		for i, elem := range b.Request {
			dst[i] = byte(elem)
		}
		req = append(req, dst...)
	}
	if b.LastFrame {
		req = append(req, byte(SubelemLastIndication), 1, 1)
	}
	return hex.EncodeToString(req), nil
}

// SendBeaconRequest sends a Beacon Request to the specified client.
func (s *Server) SendBeaconRequest(ctx context.Context, clientMAC string, param BeaconReqParams) error {
	beaconReqStr, err := param.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize beacon request")
	}
	args := []string{"-p" + s.ctrlPath(), "REQ_BEACON", clientMAC, beaconReqStr}
	if err := s.host.Command(hostapdCLI, args...).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to send Beacon Request to client %s", clientMAC)
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

// STAInfo contains the information of a connected STA.
type STAInfo struct {
	// RxPackets is the count of Rx packets.
	RxPackets int
	// RxPackets is the count of Tx packets.
	TxPackets int
	// RxPackets is the size of Rx data.
	RxBytes int
	// RxPackets is the size of Tx data.
	TxBytes int
	// ConnectedTime is the connected time of the STA.
	ConnectedTime time.Duration
	// InactiveTime is the inactive time of the STA.
	InactiveTime time.Duration
}

// parseSTAInfo parses the output of hostapd_cli "sta" command.
func parseSTAInfo(ctx context.Context, staMAC, payload string) (*STAInfo, error) {
	ret := &STAInfo{}

	lines := strings.Split(strings.TrimSpace(payload), "\n")
	if len(lines) == 0 {
		return nil, errors.New("empty STA info payload")
	}
	if lines[0] != staMAC {
		return nil, errors.Errorf("invalid STA info, first line should be MAC, got %s, want %s", lines[0], staMAC)
	}

	// Drop the first line (station MAC) and start parsing.
	lines = lines[1:]

	type parserType func(string) error

	intParser := func(out *int) parserType {
		return func(in string) error {
			result, err := strconv.Atoi(in)
			if err != nil {
				return err
			}
			*out = result
			return nil
		}
	}
	durationParser := func(out *time.Duration, unit time.Duration) parserType {
		return func(in string) error {
			result, err := strconv.Atoi(in)
			if err != nil {
				return err
			}
			*out = time.Duration(result) * unit
			return nil
		}
	}

	parsers := map[string]parserType{
		"rx_packets":     intParser(&ret.RxPackets),
		"tx_packets":     intParser(&ret.TxPackets),
		"rx_bytes":       intParser(&ret.RxBytes),
		"tx_bytes":       intParser(&ret.TxBytes),
		"connected_time": durationParser(&ret.ConnectedTime, time.Second),
		"inactive_msec":  durationParser(&ret.InactiveTime, time.Millisecond),
	}

	for _, line := range lines {
		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) < 2 {
			// Log and skip the lines with no "=".
			testing.ContextLogf(ctx, "Unexpected STA info line without '=': %s", line)
			continue
		}
		name, content := tokens[0], tokens[1]
		parser, ok := parsers[name]
		if !ok {
			// Unused field, skip.
			continue
		}
		if err := parser(content); err != nil {
			return nil, errors.Wrapf(err, "failed to parse field %s", name)
		}
	}
	return ret, nil
}

// STAInfo queries the information of the connected STA.
func (s *Server) STAInfo(ctx context.Context, staMAC string) (*STAInfo, error) {
	payload, err := s.hostapdCLI(ctx, "sta", staMAC)
	if err != nil {
		return nil, err
	}
	payload = strings.TrimSpace(payload)
	if payload == "FAIL" {
		return nil, errors.Errorf("failed to query STA with MAC=%s", staMAC)
	}
	return parseSTAInfo(ctx, staMAC, payload)
}
