// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"chromiumos/tast/errors"
)

// Conns simply wraps a list of Conn and provides a method to Close all of them.
type Conns []*Conn

// Close closes all of the connections.
func (cs Conns) Close() error {
	var firstErr error
	numErrs := 0
	for _, c := range cs {
		if err := c.Close(); err != nil {
			numErrs++
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if numErrs == 0 {
		return nil
	}
	if numErrs == 1 {
		return firstErr
	}
	return errors.Wrapf(firstErr, "failed closing multiple connections: encountered %d errors; first one follows", numErrs)
}
