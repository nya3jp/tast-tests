// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	// Used to embed api_wrapper.js in string variable `systemDataProviderJs`.
	_ "embed"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// systemDataProviderJs is a stringified JS file that exposes the SystemDataProvider mojo
// API.
//go:embed api_wrapper.js
var systemDataProviderJs string

// MojoAPI is a struct that encapsulates a SystemDataProvider mojo remote.
type MojoAPI struct {
	conn       *chrome.Conn
	mojoRemote *chrome.JSObject
}

// SystemDataProviderMojoAPI returns a MojoAPI object that is connected to a SystemDataProvider
// mojo remote instance on success, or an error.
func SystemDataProviderMojoAPI(ctx context.Context, conn *chrome.Conn) (*MojoAPI, error) {
	var mojoRemote chrome.JSObject
	if err := conn.Call(ctx, &mojoRemote, systemDataProviderJs); err != nil {
		return nil, errors.Wrap(err, "failed to set up the SystemDataProvider mojo API")
	}

	return &MojoAPI{conn, &mojoRemote}, nil
}

// RunFetchSystemInfo calls into the injected SystemDataProvider mojo API.
func (m *MojoAPI) RunFetchSystemInfo(ctx context.Context) error {
	jsWrap := "function() { return this.fetchSystemInfo() }"
	if err := m.mojoRemote.Call(ctx, nil, jsWrap); err != nil {
		return errors.Wrap(err, "failed to run fetchSystemInfo")
	}

	return nil
}

// Release frees the resources help by the internal MojoAPI components.
func (m *MojoAPI) Release(ctx context.Context) error {
	return m.mojoRemote.Release(ctx)
}
