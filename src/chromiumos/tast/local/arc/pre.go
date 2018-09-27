// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

var bootedPre *BootedPre

func init() {
	bootedPre = &BootedPre{}
}

func Booted() *BootedPre { return bootedPre }

type BootedPre struct {
	cr  *chrome.Chrome
	arc *ARC
}

func (p *BootedPre) ARC() *ARC { return p.arc }

func (p *BootedPre) Chrome() *chrome.Chrome { return p.cr }

// Prepare is defined to implement testing.Precondition.
// It is called by the test framework at the beginning of every test using this precondition.
func (p *BootedPre) Prepare(ctx context.Context) error {
	if p.arc != nil {
		// TODO: Check that it's still usable.
		return nil
	}

	var err error
	if p.cr, err = chrome.New(ctx, chrome.ARCEnabled()); err != nil {
		return fmt.Errorf("failed to start Chrome: %v", err)
	}
	if p.arc, err = New(ctx, testing.ContextOutDir(ctx)); err != nil {
		p.cr.Close(ctx)
		p.cr = nil
		return fmt.Errorf("failed to start ARC: %v", err)
	}
	return nil
}

func (p *BootedPre) String() string { return "arc_booted" }
