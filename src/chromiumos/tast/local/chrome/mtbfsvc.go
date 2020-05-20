// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// SvcLoginReusePre implements New and Close for precondition.
type SvcLoginReusePre struct {
	// S is the service state
	S *testing.ServiceState
	// CR is the chrome instance
	CR *Chrome
}

// PrePrepare prepares the procondition.
func (p *SvcLoginReusePre) PrePrepare(ctx context.Context) error {
	if p.CR != nil {
		//TODO: check the current cr usability
		return nil
	}

	var userID, userPasswd string
	var ok bool
	if userID, ok = p.S.Var("userID"); !ok {
		return errors.New("UserID variable is not set")
	}
	if userPasswd, ok = p.S.Var("userPasswd"); !ok {
		return errors.New("userPasswd variable is not set")
	}
	var opts []Option // Options that should be passed to New.
	opts = append(
		opts, KeepState(),
		ARCSupported(),
		ReuseLogin(),
		Auth(userID, userPasswd, ""))

	cr, err := New(ctx, opts...)
	if err != nil {
		return err
	}
	p.CR = cr
	return nil
}

// PreClose closes the precondition resources.
func (p *SvcLoginReusePre) PreClose(ctx context.Context) error {
	if p.CR == nil {
		return nil
	}
	err := p.CR.Close(ctx)
	p.CR = nil
	return err
}
