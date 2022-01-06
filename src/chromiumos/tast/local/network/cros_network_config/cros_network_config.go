// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netconf contains the mojo connection to cros_network_config.
package netconf

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// CrosNetworkConfig contains the mojo connection to cros_network_config.
type CrosNetworkConfig struct {
	conn       *chrome.Conn
	mojoRemote *chrome.JSObject
}

// NewCrosNetworkConfig creates a connection to cros_network_config that allows
// to make mojo calls. Only works in a context where chrome://network may be
// opened.
func NewCrosNetworkConfig(ctx context.Context, cr *chrome.Chrome) (*CrosNetworkConfig, error) {
	conn, err := cr.NewConn(ctx, "chrome://network")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open network tab")
	}

	var mojoRemote chrome.JSObject
	if err := conn.Call(ctx, &mojoRemote, crosNetworkConfigJs); err != nil {
		return nil, errors.Wrap(err, "failed to set up the network mojo API")
	}

	return &CrosNetworkConfig{conn, &mojoRemote}, nil
}

// Close cleans up the injected javascript and closes the chrome://network tab.
func (c *CrosNetworkConfig) Close(ctx context.Context) error {
	if err := c.mojoRemote.Release(ctx); err != nil {
		return err
	}
	return c.conn.Close()
}

// GetManagedProperties returns the managed properties of the given network,
// managed properties contain information on which values are set by policy or
// user. Look at cros_network_config.mojom or onc_spec.md for more information.
func (c *CrosNetworkConfig) GetManagedProperties(ctx context.Context, guid string) (*ManagedProperties, error) {
	var result ManagedProperties

	if err := c.mojoRemote.Call(ctx, &result, "function(guid) {return this.getManagedProperties(guid)}", guid); err != nil {
		return nil, errors.Wrap(err, "failed to call cros_network_config javascript wrapper")
	}

	return &result, nil
}

// ConfigureNetwork either configures a new network or updates an existing
// network configuration.
func (c *CrosNetworkConfig) ConfigureNetwork(ctx context.Context, properties ConfigProperties, shared bool) (string, error) {

	var result struct {
		GUID         string
		ErrorMessage string
	}

	if err := c.mojoRemote.Call(ctx, &result,
		"function(properties, shared) {return this.configureNetwork(properties, shared)}", properties, shared); err != nil {
		return "", errors.Wrap(err, "failed to call cros_network_config javascript wrapper")
	}

	if result.ErrorMessage != "" {
		return "", errors.New(result.ErrorMessage)
	}

	return result.GUID, nil
}

// ForgetNetwork removes the network with guid from the device.
func (c *CrosNetworkConfig) ForgetNetwork(ctx context.Context, guid string) (bool, error) {
	var result struct{ Success bool }
	if err := c.mojoRemote.Call(ctx, &result,
		"function(guid) {return this.forgetNetwork(guid)}", guid); err != nil {
		return false, errors.Wrap(err, "failed to run forgetNetwork")
	}
	return result.Success, nil
}

// Stores cros network config remote in between calls. This can be exported to
// a file as soon as file dependencies may be passed to grpc services.
const crosNetworkConfigJs = `
/**
 * @fileoverview A wrapper file for the cros network config API.
 */
async function() {
  return {
    crosNetworkConfig_: null,

    getCrosNetworkConfig() {
      if (!this.crosNetworkConfig_) {
				this.crosNetworkConfig_ =
				    chromeos.networkConfig.mojom.CrosNetworkConfig.getRemote();
      }
      return this.crosNetworkConfig_;
		},

		async getManagedProperties(guid) {
			response = await this.getCrosNetworkConfig().getManagedProperties(guid);

			// Delete mojo uint64 typed properties because BigInt cannot be
			// serialized.
			delete response.result.trafficCounterResetTime;

			return response.result;
		},

		async configureNetwork(properties, shared) {
			return await this.getCrosNetworkConfig().configureNetwork(properties, shared);
		},

		async forgetNetwork(guid) {
			return await this.getCrosNetworkConfig().forgetNetwork(guid);
		},
	}
}
`
