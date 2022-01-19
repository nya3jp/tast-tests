// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package openwrt

import (
	"context"

	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/ssh"
)

type openWrtRouter struct {
	router.BaseRouterStruct
}

// NewOpenWrtRouter prepares initial test AP state (e.g., initializing wiphy/wdev).
// ctx is the deadline for the step and daemonCtx is the lifetime for background
// daemons.
func NewOpenWrtRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (*openWrtRouter, error) {
	// TODO
	return nil, nil
}

func (o openWrtRouter) Close(ctx context.Context) error {
	// TODO
	panic("implement me")
}

func (o openWrtRouter) RouterType() router.Type {
	// TODO
	panic("implement me")
}
