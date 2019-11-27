// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
)

// Port from Brian's PoC crrev.com/c/1733740

// HostAPConfig is the config used to start hostapd on router.
// TODO: should Iface be auto-selected wrt the setting like channel no.?
type HostAPConfig struct {
	Ssid string
}

// Format the config into hostapd.conf format.
func (c *HostAPConfig) Format(iface string) string {
	configMap := map[string]string{
		"ssid":      c.Ssid,
		"interface": iface,
		// TODO: not yet see any usage, keep it simple for now.
		// "ctrl_interface": ctrlPath,

		"logger_syslog":       "-1",
		"logger_syslog_level": "0",
		// default RTS and frag threshold to "off"
		"rts_threshold":   "2347",
		"fragm_threshold": "2346",
		"driver":          "nl80211",

		// TODO: parameterize these.
		"hw_mode":     "g",
		"channel":     "6",
		"ieee80211n":  "1",
		"ht_capab":    "[HT40+]",
		"wmm_enabled": "1",
	}
	s := ""
	for key, val := range configMap {
		s = s + fmt.Sprintf("%s=%s\n", key, val)
	}
	return s
}

// HostAPOption is the type used to specify options of HostAPConfig.
type HostAPOption func(*HostAPConfig)

// NewHostAPConfig creates a HostAPConfig with given options.
func NewHostAPConfig(ssid string, ops ...HostAPOption) *HostAPConfig {
	conf := &HostAPConfig{
		Ssid: ssid,
	}
	for _, op := range ops {
		op(conf)
	}
	return conf
}

// HostAPServer is the object to control the hostapd on router.
type HostAPServer struct {
	host     *dut.DUT
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
	done := make(chan error, 1)
	go func() {
		var msg []byte
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
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
			// TODO: check if the command terminates unexpectedly.
			//       but it seems the stdout will be closed on exit
			//       so maybe the err on stdout.Read is it.
		}
		// Service ready, free resources.
		close(done)
		msg = nil
		// Drain the remaining stdout till EOF.
		// TODO: is it ok to just close the pipe? (SIGPIPE?)
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
	return nil
}

// Stop the HostAPServer.
func (ap *HostAPServer) Stop(ctx context.Context) error {
	// TODO: remove config file.
	if ap.cmd == nil {
		return errors.New("server not started")
	}
	ap.cmd.Abort()
	// TODO: This always has error as it is aborted. Is this really meaningful?
	err := ap.cmd.Wait(ctx)
	ap.cmd = nil
	return err
}
