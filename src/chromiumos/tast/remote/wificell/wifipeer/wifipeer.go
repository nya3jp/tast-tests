// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wifipeer builds and controlls peer chrome OS devices for wifi tests.
package wifipeer

import (
	"context"
	"fmt"

	"chromiumos/tast/dut"
	"chromiumos/tast/ssh"
)

// WifiPeer holds the structure of a Chrome OS peer device for wifi tests.
type WifiPeer struct {
	Peers []*ssh.Conn
}

// MakePeers constructs the peer devices needed for the tests.
func (peer *WifiPeer) MakePeers(ctx context.Context, testdut *dut.DUT,
	count int) (_ []*ssh.Conn, retErr error) {
	fmt.Println("MP-->2:", testdut)
	for i := 0; i < count; i++ {
		newDut, err := testdut.DefaultWifiPeerHost(ctx, i)
		fmt.Println(err)
		fmt.Println(newDut)
		peer.peers = append(peer.peers, newDut)
		return nil, nil
		fmt.Println(peer.peers)
	}
	return peer.peers, nil //TODO(hinton): Change nil to error.
}
