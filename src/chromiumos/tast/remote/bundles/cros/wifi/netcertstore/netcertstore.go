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

// CreateStore creates a new NetCertStore.
// On success, it returns the cert store, a shortenedCtx (so that there's enough time for cleanup at the end)
// and a cleanup function.
// Note that it is the responsibility of caller to call cleanupFunc.
func CreateStore(ctx context.Context, s *testing.State) (shortenedCtx context.Context, cleanupFunc func(), store *netcertstore.Store, retErr error) {
	r, err := hwsec.NewCmdRunner(s.DUT())
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to create hwsec.CmdRunner")
	}

	store, err = netcertstore.CreateStore(ctx, r)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to create NetCertStore")
	}

	// We need 5 seconds to cleanup.
	shortenedCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)

	cleanupFunc = func() {
		store.Cleanup(ctx)
		cancel()
	}

	return shortenedCtx, cleanupFunc, store, nil
}
