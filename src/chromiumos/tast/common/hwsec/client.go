// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type BaseClient struct {
	r Runner
}

func NewBaseClient(r Runner) *BaseClient {
	return &BaseClient{r}
}

func (c *BaseClient) EnsureTpmIsReady(ctx context.Context) error {
	panic("not implemented; this method is available on both LocalClient and RemoteClient")
}
