// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

import (
	"context"
	"fmt"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Ax contains the funcionality that the ax testbed router should support.
type Ax interface {
	router.Base
	// RouterIP gets the router's IP address.
	RouterIP(ctx context.Context) (string, error)
	// ApplyRouterSettings takes in the router config parameters, stages them, and then restarts the wireless service to have the changes realized on the router.
	ApplyRouterSettings(ctx context.Context, cfg *Config) error
	// RetrieveConfiguration snapshots the router's current configuration from the RAM from the stdout of "nvram show" into a string.
	RetrieveConfiguration(ctx context.Context) (string, error)
	// UpdateConfig saves and applies the configuration to the router.
	UpdateConfig(ctx context.Context, recoveryMap map[string]ConfigParam) error
}

// axRouterStruct is used to control the ax wireless router and stores state of the router.
type axRouterStruct struct {
	router.BaseRouterStruct
}

// NewAxRouter prepares initial test AP state.
func NewAxRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (*axRouterStruct, error) {
	r := &axRouterStruct{
		router.BaseRouterStruct{
			Host:  host,
			Name:  name,
			Rtype: router.AxT,
		},
	}
	return r, nil
}

// stageRouterParam changes the router configuration in memory. The actual router configuration does not take effect until restartWirelessService is invoked which pulls the configuration from memory.
func (r *axRouterStruct) stageRouterParam(ctx context.Context, band BandEnum, key NVRAMKeyEnum, value string) error {
	return r.Host.CommandContext(ctx, "/bin/nvram", "set", fmt.Sprintf("%s_%s=%s", band, key, value)).Run()
}

// restartWirelessService restarts the router's WiFi service, updating its config with the staged changes.
func (r *axRouterStruct) restartWirelessService(ctx context.Context) error {
	testing.ContextLog(ctx, "Restarting Wireless Service")
	return r.Host.CommandContext(ctx, "/sbin/service", RestartWirelessService).Run()
}

// originalValue takes the NVRAM state snapshotted at the beginning of the test, and extracts the value associated with the key of the setting that's passed in.
func (r *axRouterStruct) originalValue(setting ConfigParam, originalSettingsString string) (ConfigParam, error) {
	re := regexp.MustCompile(fmt.Sprintf("%s_%s=(?P<value>[a-zA-Z0-9]*)", setting.Band, setting.Key))
	matches := re.FindAllStringSubmatch(originalSettingsString, -1)
	for _, m := range matches {
		return ConfigParam{setting.Band, setting.Key, m[1]}, nil
	}
	return ConfigParam{}, errors.New("could not find default value")
}

// ApplyRouterSettings will take in the router config parameters, stage them, and then restart the wireless service to have the changes realized on the router.
func (r *axRouterStruct) ApplyRouterSettings(ctx context.Context, cfg *Config) error {
	for _, setting := range cfg.RouterConfigParams {
		if _, ok := cfg.RouterRecoveryMap[string(setting.Key)]; !ok {
			res, err := r.originalValue(setting, *cfg.NVRAMOut)
			if err != nil {
				return errors.Wrapf(err, "could not get default value of parameter %s_%s", setting.Band, setting.Key)
			}
			cfg.RouterRecoveryMap[string(setting.Key)] = res
		}
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

// RouterType returns the router's type.
func (r *axRouterStruct) RouterType() router.Type {
	return r.Rtype
}

// RouterIP gets the ax router's ip.
func (r *axRouterStruct) RouterIP(ctx context.Context) (string, error) {
	ifcfgOut, err := r.Host.CommandContext(ctx, "/sbin/ifconfig", RouterIface).Output()
	if err != nil {
		return "", errors.Wrap(err, string(ifcfgOut))
	}
	inetRegexp := regexp.MustCompile(`inet addr:(?P<ipaddr>(?:[0-9]{1,3}\.){3}[0-9]{1,3})`)
	match := inetRegexp.FindStringSubmatch(string(ifcfgOut))
	if match == nil {
		return "", errors.Errorf("failed to get inet address on iface %s with output %s", RouterIface, string(ifcfgOut))
	} else if len(match) != 2 {
		return "", errors.Errorf("got unexpected number of inet addresses. Expected 2, got %d", len(match))
	}
	return match[1], nil
}

// RetrieveConfiguration snapshots the current router's configuration in ram into a string.
func (r *axRouterStruct) RetrieveConfiguration(ctx context.Context) (string, error) {
	nvramOut, err := r.Host.CommandContext(ctx, "/bin/nvram", "show").Output()
	if err != nil {
		errors.Wrap(err, string(nvramOut))
	}
	return string(nvramOut), nil
}

// UpdateConfig saves and applies the configuration to the router.
func (r *axRouterStruct) UpdateConfig(ctx context.Context, recoveryMap map[string]ConfigParam) error {
	defer r.restartWirelessService(ctx)
	paramList := make([]ConfigParam, 0, len(recoveryMap))
	testing.ContextLog(ctx, "Recovery map: ", recoveryMap)
	for _, val := range recoveryMap {
		paramList = append(paramList, val)
	}
	for _, setting := range paramList {
		if err := r.stageRouterParam(ctx, setting.Band, setting.Key, setting.Value); err != nil {
			return errors.Errorf("failed to set %s_%s=%s", setting.Band, setting.Key, setting.Value)
		}
	}
	return nil
}
