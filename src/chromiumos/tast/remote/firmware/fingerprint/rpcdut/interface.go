// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpcdut

import (
	"chromiumos/tast/ssh"
	"context"
)

// DUTConnector wraps the basic methods of used to control a dut using dut.DUT.
//
// This is used for functions that don't need the RPC connection provided by
// rpcdut.RPCDUT.
type DUTConnector interface {
	Close(ctx context.Context) error
	// CompanionDeviceHostname(suffix string) (string, error)
	Conn() *ssh.Conn
	Connect(ctx context.Context) error
	Connected(ctx context.Context) bool
	// DefaultCameraboxChart(ctx context.Context) (*ssh.Conn, error)
	// DefaultWifiPcapHost(ctx context.Context) (*ssh.Conn, error)
	// DefaultWifiRouterHost(ctx context.Context) (*ssh.Conn, error)
	Disconnect(ctx context.Context) error
	// GetFile(ctx context.Context, src, dst string) error
	HostName() string
	KeyDir() string
	KeyFile() string
	// NewSecondaryDevice(target string) (*DUT, error)
	Reboot(ctx context.Context) error
	// WaitConnect(ctx context.Context) error
	WaitUnreachable(ctx context.Context) error
	// WifiPeerHost(ctx context.Context, index int) (*ssh.Conn, error)
}
