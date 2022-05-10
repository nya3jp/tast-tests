// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wifipeer builds and controls peer ChromeOS devices for Wi-Fi tests.
package wifipeer

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// MakePeers constructs the peer devices needed for the tests.
func MakePeers(ctx context.Context, testdut *dut.DUT, count int) (peers []*ssh.Conn, retErr error) {
	defer func() {
		if retErr != nil {
			for _, peer := range peers {
				peer.Close(ctx)
			}
		}
	}()
	for i := 1; i <= count; i++ {
		peer, err := testdut.WifiPeerHost(ctx, i)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connet to the peer %d", i)
		}
		peers = append(peers, peer)
	}
	return peers, nil
}
