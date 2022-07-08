// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"

	"chromiumos/tast/ssh"
)

type WiFiHost interface {
	Conn() *ssh.Conn
	// CollectLogs()
	Close(ctx context.Context) error
	Name() string
	PingHost(*WiFiHost)
	PingIP(*string)
	ArPing()
	IPv4Addrs(ctx context.Context) ([]net.IP, error)
	HwAddr()
	Pcap()
}

type WiFiHostImpl struct {
	// conn *ssh.Conn
}

// func (h *WiFiHostImpl) Conn() *ssh.Conn {
// 	return h.conn
// }
