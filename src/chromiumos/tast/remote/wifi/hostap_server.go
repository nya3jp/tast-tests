// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/testing"
)

// Port from Brian's PoC crrev.com/c/1733740

// HWMode is the type for specifying hostap hw mode.
type HWMode string

// HWMode enums.
const (
	HWMode80211a  HWMode = "a"
	HWMode80211b  HWMode = "b"
	HWMode80211g  HWMode = "g"
	HWMode80211ad HWMode = "ad"
)

// HostAPConfig is the config used to start hostapd on router.
type HostAPConfig struct {
	Ssid    string
	HWMode  HWMode
	Channel int
}

// Format the config into hostapd.conf format.
func (c *HostAPConfig) Format(iface string) string {
	configMap := map[string]string{
		"ssid":      c.Ssid,
		"interface": iface,
		"hw_mode":   string(c.HWMode),
		"channel":   strconv.Itoa(c.Channel),

		// TODO: not yet use hostapd_cli, keep it simple for now.
		// "ctrl_interface": ctrlPath,
		"logger_syslog":       "-1",
		"logger_syslog_level": "0",
		// default RTS and frag threshold to "off"
		"rts_threshold":   "2347",
		"fragm_threshold": "2346",
		"driver":          "nl80211",

		// TODO: parameterize these.
		"ieee80211n":  "1",
		"ht_capab":    "[HT40+]",
		"wmm_enabled": "1",
	}
	var builder strings.Builder
	for key, val := range configMap {
		builder.WriteString(fmt.Sprintf("%s=%s\n", key, val))
	}
	return builder.String()
}

// TODO: As we have options, maybe it's better to give hostap a independent package,
// and the names can be simpler.

// HostAPOption is the type used to specify options of HostAPConfig.
type HostAPOption func(*HostAPConfig)

// HostAPHWMode returns a HostAPConfig which sets hw_mode in hostapd config.
func HostAPHWMode(mode HWMode) HostAPOption {
	return func(c *HostAPConfig) {
		c.HWMode = mode
	}
}

// HostAPChannel returns a HostAPConfig which sets channel in hostapd config.
func HostAPChannel(ch int) HostAPOption {
	return func(c *HostAPConfig) {
		c.Channel = ch
	}
}

// NewHostAPConfig creates a HostAPConfig with given options.
func NewHostAPConfig(ssid string, ops ...HostAPOption) *HostAPConfig {
	// Default config.
	conf := &HostAPConfig{
		Ssid:    ssid,
		HWMode:  HWMode80211g,
		Channel: 1,
	}
	for _, op := range ops {
		op(conf)
	}
	return conf
}

// HostAPServer is the object to control the hostapd on router.
type HostAPServer struct {
	host     *dut.DUT // TODO(crbug.com/1019537): use a more suitable ssh object instead of DUT. DUT is specific for Chromebook.
	conf     *HostAPConfig
	confPath string
	iface    string
	cmd      *host.Cmd
}

// NewHostAPServer creates a new HostAPServer and runs hostapd on the router.
func NewHostAPServer(ctx context.Context, r *Router, iface string, c *HostAPConfig) (*HostAPServer, error) {
	server := &HostAPServer{
		host:  r.host,
		conf:  c,
		iface: iface,
	}
	if err := server.start(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (ap *HostAPServer) start(ctx context.Context) error {
	ap.confPath = fmt.Sprintf("/tmp/hostapd-%s.conf", ap.iface)

	err := writeToDUT(ctx, ap.host, ap.confPath, []byte(ap.conf.Format(ap.iface)))
	if err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLog(ctx, "Starting hostapd")
	cmd := ap.host.Command("hostapd", "-dd", "-t", ap.confPath)
	// TODO: Combine stdout and stderr?
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StdoutPipe of hostapd")
	}
	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	ap.cmd = cmd

	// Wait for hostapd to get ready.
	// TODO: All debug log goes through SSH once, it may be wasteful. Not sure if there's
	// a better way to wait until hostapd ready.
	done := make(chan error, 1)
	go func() {
		var msg []byte
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				// If the program exits unexpectedly, it will also go here with
				// err = io.EOF.
				done <- errors.Wrap(err, "failed to read stdout of hostapd")
				return
			}
			msg = append(msg, buf[:n]...)
			s := string(msg)
			if strings.Contains(s, "Setup of interface done") {
				break
			}
			if strings.Contains(s, "Interface initialization failed") {
				// Don't keep polling. We failed.
				done <- errors.New("hostapd failed to initialize AP interface")
			}
		}
		// Service ready, free resources.
		close(done)
		msg = nil
		// Drain the remaining stdout till EOF.
		// We cannot just close the pipe or else the writer will be blocked and write call
		// in remote program gets blocked in the end.
		for {
			_, err = stdout.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	select {
	case err = <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err != nil {
		ap.Stop(ctx)
		return err
	}
	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// Stop the HostAPServer.
func (ap *HostAPServer) Stop(ctx context.Context) error {
	if ap.cmd == nil {
		return errors.New("server not started")
	}

	ap.cmd.Abort()
	// Skip the error in Wait as the process is aborted, and always has error in wait.
	ap.cmd.Wait(ctx)
	ap.cmd = nil
	if err := ap.host.Command("rm", ap.confPath).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to remove config with err=%s", err.Error())
	}
	return nil
}
