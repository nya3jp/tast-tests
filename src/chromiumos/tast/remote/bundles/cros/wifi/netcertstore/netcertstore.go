// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netcertstore holds a simple wrapper that abstracts away the creation of NetCertStore. Actual NetCertStore code is located in chromiumos/tast/common/pkcs11/netcertstore.
package netcertstore

import (
	"context"
	"time"

	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// NewNetCertStore creates a new NetCertStore.
// On success, it returns the cert store, a shortenedCtx (so that there's enough time for cleanup at the end)
// and a cleanup function.
// Note that it is the responsibility of caller to call cleanupFunc.
func NewNetCertStore(ctx context.Context, s *testing.State) (shortenedCtx context.Context, cleanupFunc func(), store *netcertstore.NetCertStore, retErr error) {
	r, retErr := hwsec.NewCmdRunner(s.DUT())
	if retErr != nil {
		return nil, nil, nil, errors.Wrap(retErr, "failed to create hwsec.CmdRunner")
	}

	store, retErr = netcertstore.NewNetCertStore(ctx, r)
	if retErr != nil {
		return nil, nil, nil, errors.Wrap(retErr, "failed to create NetCertStore")
	}

	// We need 5 seconds to cleanup.
	shortenedCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)

	cleanupFunc = func() {
		netcertstore.CleanupNetCertStore(shortenedCtx)
		cancel()
	}

	return shortenedCtx, cleanupFunc, store, nil
}
