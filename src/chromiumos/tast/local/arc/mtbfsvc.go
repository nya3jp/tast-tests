// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// NewForRPC is called by gRPC services to make sure ARC is up and running.
func NewForRPC(ctx context.Context, outDir string) (*SvcLoginReusePre, error) {
	res := &SvcLoginReusePre{}
	var err error

	// A temporary structure to use its interfaces.
	p := &preImpl{
		name:    loginReusePreName,
		timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
		pi:      &preLoginReuse{},
	}

	if res.arc, err = New(ctx, outDir); err != nil {
		testing.ContextLog(ctx, "Failed to start ARC: ", err)
		return res, err
	}
	if res.origInitPID, err = InitPID(); err != nil {
		testing.ContextLog(ctx, "Failed to get initial init PID: ", err)
		return res, err
	}
	if res.origInstalledPkgs, err = p.installedPackages(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to list initial packages: ", err)
		return res, err
	}
	if res.origRunningPkgs, err = p.runningPackages(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to list running packages: ", err)
		return res, err
	}
	return res, err
}

// SvcLoginReusePre is used for gRPC service ARC login.
type SvcLoginReusePre struct {
	arc *ARC

	origInitPID       int32               // initial PID (outside container) of ARC init process
	origInstalledPkgs map[string]struct{} // initially-installed packages
	origRunningPkgs   map[string]struct{} // initially-running packages
}

// Close closes the internal preImpl.
func (p *SvcLoginReusePre) Close() error {
	if p.arc != nil {
		if err := p.arc.Close(); err != nil {
			return err
		}
		p.arc = nil
	}
	return nil
}
