// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// WiFiHost interface describes parts of interface common between DUT and Router.
type WiFiHost interface {
	// ArPingIP sends ARP ping from host to target IP.
	ArPingIP(ctx context.Context, targetIP string, opts ...arping.Option) (*arping.Result, error)
	// Close closes connection to DUT/Router.
	Close(ctx context.Context) error
	// Conn returns SSH connection object to a given host.
	Conn() *ssh.Conn
	// Name returns host name.
	Name() string
	// PingIP sends ICMP ping from host to target IP.
	PingIP(ctx context.Context, targetIP string, opts ...ping.Option) (*ping.Result, error)
	// HwAddr returns Hardware address of the wifi-related interface.
	HwAddr(ctx context.Context) (net.HardwareAddr, error)
	// IPv4Addrs returns IPv4 addresses of the wifi-related interface.
	IPv4Addrs(ctx context.Context) ([]net.IP, error)
}

// Ping sends ping between two hosts.
func Ping(ctx context.Context, srcHost, dstHost WiFiHost, opts ...ping.Option) (*ping.Result, error) {
	addrs, err := dstHost.IPv4Addrs(ctx)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, errors.New("Empty address returned")
	}
	return srcHost.PingIP(ctx, addrs[0].String(), opts...)
}

// ArPing sends ARP ping between two hosts.
func ArPing(ctx context.Context, srcHost, dstHost WiFiHost, opts ...arping.Option) (*arping.Result, error) {
	addrs, err := dstHost.IPv4Addrs(ctx)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, errors.New("Empty address returned")
	}
	return srcHost.ArPingIP(ctx, addrs[0].String(), opts...)
}

// VerifyPingResults checks if ping results are within acceptable range.
func VerifyPingResults(res interface{}, lossThreshold float64) error {
	var loss float64
	switch t := res.(type) {
	case ping.Result:
		loss = res.(ping.Result).Loss
	case *ping.Result:
		loss = res.(*ping.Result).Loss
	case arping.Result:
		loss = res.(arping.Result).Loss
	case *arping.Result:
		loss = res.(*arping.Result).Loss
	default:
		return errors.Errorf("unknown result type %T", t)
	}

	if loss > lossThreshold {
		return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", loss, pingLossThreshold)
	}

	return nil
}
