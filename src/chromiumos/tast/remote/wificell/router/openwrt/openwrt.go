// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package openwrt

import (
	"context"

	"chromiumos/tast/remote/wificell/router/common"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/ssh"
)

// Router controls an OpenWrt router and stores the router state.
type Router struct {
	common.BaseRouterStruct
}

// NewRouter prepares initial test AP state (e.g., initializing wiphy/wdev).
// ctx is the deadline for the step and daemonCtx is the lifetime for background
// daemons.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (*Router, error) {
	// TODO
	return nil, nil
}

func (r Router) Close(ctx context.Context) error {
	// TODO
	panic("implement me")
}

func (r Router) RouterType() support.RouterType {
	// TODO
	panic("implement me")
}
