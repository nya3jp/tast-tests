// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
)

// Port holds the chameleon board port information.
type Port struct {
	ch *Chameleon
	id int
}

// NewPort creates a new port object for the chameleon board.
func (ch *Chameleon) NewPort(ctx context.Context, portID int) (*Port, error) {
	supported := false
	for _, p := range ch.ports {
		if p == portID {
			supported = true
			break
		}
	}
	if !supported {
		return nil, errors.Errorf("port %d is not supported on the chameleon board", portID)
	}
	return &Port{ch: ch, id: portID}, nil
}

// Plug calls the Chameleon plug method.
func (p *Port) Plug(ctx context.Context) error {
	return p.ch.xmlrpc.Run(ctx, xmlrpc.NewCall("Plug", p.id))
}

// Unplug calls the Chameleon Unplug method.
func (p *Port) Unplug(ctx context.Context) error {
	return p.ch.xmlrpc.Run(ctx, xmlrpc.NewCall("Unplug", p.id))
}

// WaitVideoInputStable calls the Chameleon WaitVideoInputStable method.
func (p *Port) WaitVideoInputStable(ctx context.Context, timeout time.Duration) error {
	t := int(math.Ceil(timeout.Seconds())) // Convert to the least integer greater or equal.
	var stable bool
	if err := p.ch.xmlrpc.Run(ctx, xmlrpc.NewCall("WaitVideoInputStable", p.id, t), &stable); err != nil {
		return err
	}
	if !stable {
		return errors.New("video input has not been stable")
	}
	return nil
}
