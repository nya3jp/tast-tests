// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/timing"
	"context"
	"fmt"
	"regexp"
)

// AxRouterConfigParam contains the information to configure a parameter on the ax router.
type AxRouterConfigParam struct {
	Band  string
	Key   string
	Value string
}

const routeriface = "br0"
const restartWirelessService = "restart_wireless"

// Wireless band selection.
const (
	Wl0 = "wl0"
	Wl1 = "wl1"
	Wl2 = "wl2"
)

// Nvram key parameters
const (
	// KeyAkm, Authentication and Key Management chooses the authentication method (e.g None (""), psk2, etc)
	KeyAkm = "akm"
	// KeyAuthMode Authentication mode (open, psk2)
	KeyAuthMode = "auth_mode_x"
	// KeySsid is the router's ssid on the chosen band
	KeySsid = "ssid"
	// KeyChanspec is the band's channel (1,2,3, etc)
	KeyChanspec = "chanspec"
	// KeyClosed is a boolean (0,1) dictating wheter the network is hidden (1 is hidden, 0 is open)
	KeyClosed = "closed"
	// KeyRadio is a boolean (0,1) dictating whether the radio is enabled (1 is enabled, 0 is disabled).
	KeyRadio = "radio"
)

// axRouterStruct is used to control the ax wireless router and stores state of the router.
type axRouterStruct struct {
	BaseRouterStruct
}

// newAxRuter prepares initial test AP state.
func newAxRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (Ax, error) {
	r := &axRouterStruct{
		BaseRouterStruct: BaseRouterStruct{
			host:  host,
			name:  name,
			rtype: AxT,
		},
	}
	shortCtx, cancel := ReserveForRouterClose(ctx)
	defer cancel()

	ctx, st := timing.Start(shortCtx, "initialize")
	defer st.End()
	return r, nil
}

// stageRouterParam changes the router configuration file. The actual router configuration isn't actually
// changed until restartWirelessService is invoked.
func (r *axRouterStruct) stageRouterParam(ctx context.Context, band, key, value string) error {
	return r.host.Command("/bin/nvram", "set", fmt.Sprintf("%s_%s=%s", band, key, value)).Run(ctx)
}

func (r *axRouterStruct) restartWirelessService(ctx context.Context) error {
	return r.host.Command("/sbin/service", restartWirelessService).Run(ctx)
}

func (r *axRouterStruct) ApplyRouterSettings(ctx context.Context, settings []AxRouterConfigParam) error {
	for _, setting := range settings {
		if err := r.stageRouterParam(ctx, setting.Band, setting.Key, setting.Value); err != nil {
			return errors.Errorf("failed to set %s_%s=%s", setting.Band, setting.Key, setting.Value)
		}
	}
	if err := r.restartWirelessService(ctx); err != nil {
		return errors.New("Faild to stage router parameters")
	}
	return nil
}

// Close cleans the resource used by Router.
func (r *axRouterStruct) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "router.Close")
	defer st.End()
	return nil
}

// GetRouterType returns the router's type.
func (r *axRouterStruct) GetRouterType() Type {
	return r.rtype
}

// GetRouterIP gets the ax router's ip.
func (r *axRouterStruct) GetRouterIP(ctx context.Context) (string, error) {
	ifcfgOut, err := r.host.Command("/sbin/ifconfig", routeriface).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, string(ifcfgOut))
	}
	inetRegexp := regexp.MustCompile(`inet addr:(?P<ipaddr>(?:[0-9]{1,3}\.){3}[0-9]{1,3})`)
	match := inetRegexp.FindStringSubmatch(string(ifcfgOut))
	if match == nil {
		return "", errors.Errorf("failed to get inet address on iface %s with output %s", routeriface, string(ifcfgOut))
	} else if len(match) != 2 {
		return "", errors.Errorf("got unexpected number of inet addresses. Expected 2, got %d", len(match))
	}
	return match[1], nil
}
