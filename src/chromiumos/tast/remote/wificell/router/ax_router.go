// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"context"
	"fmt"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// axRouterStruct is used to control the ax wireless router and stores state of the router.
type axRouterStruct struct {
	BaseRouterStruct
}

// newAxRouter prepares initial test AP state.
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
func (r *axRouterStruct) stageRouterParam(ctx context.Context, band BandEnum, key NvramKeyEnum, value string) error {
	return r.host.Command("/bin/nvram", "set", fmt.Sprintf("%s_%s=%s", band, key, value)).Run(ctx)
}

// restartWirelessService restarts the router's wifi service, updating its config with the staged changes.
func (r *axRouterStruct) restartWirelessService(ctx context.Context) error {
	testing.ContextLog(ctx, "Restarting Wireless Service")
	return r.host.Command("/sbin/service", restartWirelessService).Run(ctx)
}

// ApplyRouterSettings will take in the router config parameters, stage them, and then restart the wireless service to have the changes realized on the router.
func (r *axRouterStruct) ApplyRouterSettings(ctx context.Context, settings []AxRouterConfigParam) error {
	for _, setting := range settings {
		if err := r.stageRouterParam(ctx, setting.Band, setting.Key, setting.Value); err != nil {
			return errors.Errorf("failed to set %s_%s=%s", setting.Band, setting.Key, setting.Value)
		}
	}
	if err := r.restartWirelessService(ctx); err != nil {
		return errors.New("failed to stage router parameters")
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

// SaveConfiguration snapshots the router's configuration in ram into a file.
func (r *axRouterStruct) SaveConfiguration(ctx context.Context) error {
	cmd := fmt.Sprintf("nvram show | sort > %s", savedConfigLocation)
	return r.host.Command("/bin/bash", "-c", cmd).Run(ctx)
}

// RestoreConfiguration takes a saved router configuration file and loads it into the ram and updates the router.
func (r *axRouterStruct) RestoreConfiguration(ctx context.Context) error {
	defer r.restartWirelessService(ctx)
	cmd := fmt.Sprintf("while read in; do nvram set '$in'; done < %s", savedConfigLocation)
	return r.host.Command("/bin/bash", "-c", cmd).Run(ctx)
}
