// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"chromiumos/tast/ssh"
	"chromiumos/tast/timing"
	"context"
)

// axRouterStruct is used to control the ax wireless router and stores state of the router.
type axRouterStruct struct {
	BaseRouterStruct
}

// newAxRuter prepares initial test AP state.
func newAxRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (Ax, error) {
	r := &axRouterStruct{
		BaseRouterStruct: BaseRouterStruct{
			host:  host,
			name:  name,
			rtype: AxT,
		},
	}
	shortCtx, cancel := ReserveForRouterClose(ctx)
	defer cancel()

	ctx, st := timing.Start(shortCtx, "initialize")
	defer st.End()
	return r, nil
}

// Close cleans the resource used by Router.
func (r *axRouterStruct) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "router.Close")
	defer st.End()
	return nil
}

// GetRouterType returns the router's type
func (r *axRouterStruct) GetRouterType() Type {
	return r.rtype
}
