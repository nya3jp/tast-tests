// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
)

type remoteRunner struct {
	d *dut.DUT
}

func (r *remoteRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.d.Command(name, args...).Output(ctx)
}

type RemoteClient struct {
	*hwsec.BaseClient
}

func NewRemoteClient(d *dut.DUT) *RemoteClient {
	return &RemoteClient{BaseClient: hwsec.NewBaseClient(&remoteRunner{d})}
}

func (c *RemoteClient) EnsureTpmIsReset(ctx context.Context) error {
	panic("not implemented; this method is available on RemoteClient only")
}
