// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ax

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/router/common"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const nvramCmd = "/bin/nvram"

// Router is used to control an Asus AX wireless router and stores state of the router.
type Router struct {
	host       *ssh.Conn
	name       string
	routerType support.RouterType
}

// NewRouter prepares initial test AP state.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (*Router, error) {
	r := &Router{
		host:       host,
		name:       name,
		routerType: support.AxT,
	}
	return r, nil
}

// RouterName returns the name of the managed router device.
func (r *Router) RouterName() string {
	return r.name
}

// RouterType returns the router's type.
func (r *Router) RouterType() support.RouterType {
	return r.routerType
}

// ReinitTestState returns the router to a clean test state.
func (r *Router) ReinitTestState(ctx context.Context) error {
	testing.ContextLogf(ctx, "ReinitTestState ignored for router %q as it not necessary for %s routers", r.RouterName(), r.RouterType().String())
	return nil
}

// stageRouterParam changes the router configuration in memory. The actual router configuration does not take effect until restartWirelessService is invoked which pulls the configuration from memory.
func (r *Router) stageRouterParam(ctx context.Context, band RadioEnum, key NVRAMKeyEnum, value string) error {
	return r.host.CommandContext(ctx, "/bin/nvram", "set", fmt.Sprintf("%s_%s=%s", band, key, value)).Run()
}

// restartWirelessService restarts the router's WiFi service, updating its config with the staged changes.
func (r *Router) restartWirelessService(ctx context.Context) error {
	testing.ContextLog(ctx, "Restarting Wireless Service")
	return r.host.CommandContext(ctx, "/sbin/service", RestartWirelessService).Run()
}

// originalValue takes the NVRAM state snapshotted at the beginning of the test, and extracts the value associated with the key of the setting that's passed in.
func (r *Router) originalValue(setting ConfigParam, originalSettingsString string) (ConfigParam, error) {
	re := regexp.MustCompile(fmt.Sprintf("%s_%s=(?P<value>[a-zA-Z0-9]*)", setting.Band, setting.Key))
	matches := re.FindAllStringSubmatch(originalSettingsString, -1)
	for _, m := range matches {
		return ConfigParam{setting.Band, setting.Key, m[1]}, nil
	}
	return ConfigParam{}, errors.New("could not find default value")
}

// ApplyRouterSettings will take in the router config parameters, stage them, and then restart the wireless service to have the changes realized on the router.
func (r *Router) ApplyRouterSettings(ctx context.Context, cfg *Config) error {
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
func (r *Router) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "router.Close")
	defer st.End()
	return nil
}

// RouterIP gets the ax router's ip.
func (r *Router) RouterIP(ctx context.Context) (string, error) {
	ifcfgOut, err := r.host.CommandContext(ctx, "/sbin/ifconfig", RouterIface).Output()
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
func (r *Router) RetrieveConfiguration(ctx context.Context) (string, error) {
	nvramOut, err := r.host.CommandContext(ctx, "/bin/nvram", "show").Output()
	if err != nil {
		errors.Wrap(err, string(nvramOut))
	}
	return string(nvramOut), nil
}

// UpdateConfig saves and applies the configuration to the router.
func (r *Router) UpdateConfig(ctx context.Context, recoveryMap map[string]ConfigParam) error {
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

// ResolveAXDeviceType attempts to resolve the router's DeviceType based on the
// wps_modelnum value returned from running "nvram show" on the host.
func (r *Router) ResolveAXDeviceType(ctx context.Context) (DeviceType, error) {
	// Retrieve wps_modelnum from host.
	wpsModelNum, err := resolveWPSModelNumFromHost(ctx, r.host)
	if err != nil {
		return -1, err
	}
	// Determine DeviceType from wps_modelnum.
	var deviceType DeviceType
	switch wpsModelNum {
	case "RT-AX92U":
		deviceType = Ax6100
	case "GT-AX11000":
		deviceType = GtAx11000
	case "GT-AXE11000":
		deviceType = GtAxe11000
	default:
		return -1, errors.Errorf("failed to deduce AX DeviceType from unknown wps_modelnum %q", wpsModelNum)
	}
	return deviceType, nil
}

// HostIsAXRouter determines whether the remote host is an AX router.
func HostIsAXRouter(ctx context.Context, host *ssh.Conn) (bool, error) {
	// Verify that the host has the nvram command.
	nvramCmdExists, err := common.HostTestPath(ctx, host, "-x", nvramCmd)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check for the existence of the command %q", nvramCmd)
	}
	if !nvramCmdExists {
		return false, nil
	}

	// Verify that the model has an "AX" in it.
	wpsModelNum, err := resolveWPSModelNumFromHost(ctx, host)
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(wpsModelNum), "ax"), nil
}

func resolveWPSModelNumFromHost(ctx context.Context, host *ssh.Conn) (string, error) {
	// Get the results of running 'nvram show' on the host.
	nvramShowResult, err := host.CommandContext(ctx, nvramCmd, "show").Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to run '%s show' on host", nvramCmd)
	}
	nvramShowResultStr := string(nvramShowResult)

	// Parse value of wps_modelnum.
	valueRegex := regexp.MustCompile("(?m)^wps_modelnum=(.+)$")
	valueMatch := valueRegex.FindStringSubmatch(nvramShowResultStr)
	if valueMatch == nil {
		return "", errors.Wrapf(err, "failed to parse wpa_modelnum from '%s show' output: %q", nvramCmd, string(nvramShowResultStr))
	}
	return valueMatch[1], nil
}
