// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// systemDataProviderJs is a stringified JS file that exposes the SystemDataProvider mojo
// API.
const systemDataProviderJs = `
/**
 * @fileoverview A wrapper file around the diagnostics API.
 */
function() {
	return {
    /**
     * SystemDataProvider mojo remote.
     * @private {?chromeos.diagnostics.mojom.SystemDataProviderRemote}
     */
    systemDataProvider_: null,

		getSystemDataProvider() {
			if (!this.systemDataProvider_) {
				const hasAshMojom = typeof ash !== "undefined" &&
                            typeof ash.diagnostics !== "undefined";
        this.systemDataProvider_ = hasAshMojom ?
						ash.diagnostics.mojom.SystemDataProvider.getRemote() :
            chromeos.diagnostics.mojom.SystemDataProvider.getRemote();
			}
			return this.systemDataProvider_;
		},

		async fetchSystemInfo() {
			const result = await this.getSystemDataProvider().getSystemInfo();
			console.log("result.systemInfo from tast: ", result.systemInfo); // For debug purpose.
			return result.systemInfo;
		}
	}
}
`

// MojoAPI is a struct that encapsulates a SystemDataProvider mojo remote.
type MojoAPI struct {
	conn       *chrome.Conn
	mojoRemote *chrome.JSObject
}

// NewMojoAPI returns a MojoAPI object that is connected to a SystemDataProvider
// mojo remote instance on success, or an error.
func NewMojoAPI(ctx context.Context, conn *chrome.Conn) (*MojoAPI, error) {
	var mojoRemote chrome.JSObject
	if err := conn.Call(ctx, &mojoRemote, systemDataProviderJs); err != nil {
		return nil, errors.Wrap(err, "failed to set up the SystemDataProvider mojo API")
	}

	return &MojoAPI{conn, &mojoRemote}, nil
}

// RunFetchSystemInfo calls into the injected SystemDataProvider mojo API.
func (m *MojoAPI) RunFetchSystemInfo(ctx context.Context) error {
	jsWrap := fmt.Sprintf("function() { return this.fetchSystemInfo() }")
	if err := m.mojoRemote.Call(ctx, nil, jsWrap); err != nil {
		return errors.Wrap(err, "failed to run fetchSystemInfo")
	}

	return nil
}

// Release frees the resources help by the internal MojoAPI components.
func (m *MojoAPI) Release(ctx context.Context) error {
	return m.mojoRemote.Release(ctx)
}
