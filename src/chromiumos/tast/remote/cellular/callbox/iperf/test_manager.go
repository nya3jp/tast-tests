// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/network/iperf"
	"chromiumos/tast/ssh"
)

// TestType is the type of cellular performance test to run.
type TestType string

const (
	// TestTypeTCPRx represents a download TCP iperf test.
	TestTypeTCPRx TestType = "tcp_rx"
	// TestTypeTCPTx represents an upload TCP iperf test.
	TestTypeTCPTx = "tcp_tx"
	// TestTypeUDPRx represents a download UDP iperf test.
	TestTypeUDPRx = "udp_rx"
	// TestTypeUDPTx represents an upload UDP iperf test.
	TestTypeUDPTx = "udp_tx"
)

const (
	testTimeMargin       = 5 * time.Second
	commandTimeoutMargin = 30 * time.Second
	defaultTime          = 15 * time.Second
	minThroughput        = 0.80
	targetThroughput     = 0.90
)

var (
	protocolMap = map[TestType]iperf.Protocol{
		TestTypeTCPRx: iperf.ProtocolTCP,
		TestTypeTCPTx: iperf.ProtocolTCP,
		TestTypeUDPRx: iperf.ProtocolUDP,
		TestTypeUDPTx: iperf.ProtocolUDP,
	}

	defaultOptions = map[TestType][]iperf.ConfigOption{
		TestTypeTCPRx: {iperf.TestTimeOption(defaultTime)},
		TestTypeTCPTx: {iperf.TestTimeOption(defaultTime)},
		TestTypeUDPRx: {iperf.TestTimeOption(defaultTime), iperf.PortCountOption(1), iperf.FetchServerResultsOption(true)},
		TestTypeUDPTx: {iperf.TestTimeOption(defaultTime), iperf.PortCountOption(1), iperf.FetchServerResultsOption(true)},
	}
)

// TestManager is a helper class that manages running different types of Iperf simulations on callboxes.
type TestManager struct {
	callbox string
	conn    *ssh.Conn
	client  *manager.CallboxManagerClient
}

// NewTestManager creates a test manager for the given callbox.
func NewTestManager(callbox string, dutConn *ssh.Conn, client *manager.CallboxManagerClient) *TestManager {
	return &TestManager{
		callbox: callbox,
		conn:    dutConn,
		client:  client,
	}
}

// CalculateExpectedThroughput returns the minimum and target throughput of a cellular simulation.
//
// Note: right now, this takes a percentage of the theoretical max throughput given the current callbox configuration.
func (c *TestManager) CalculateExpectedThroughput(ctx context.Context, testType TestType) (iperf.BitRate, iperf.BitRate, error) {
	throughputResp, err := c.client.FetchMaxThroughput(ctx, &manager.FetchMaxThroughputRequestBody{Callbox: c.callbox})
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to query throughput from callbox manager")
	}

	up := iperf.BitRate(throughputResp.Uplink) * iperf.Mbps
	down := iperf.BitRate(throughputResp.Downlink) * iperf.Mbps
	switch testType {
	case TestTypeUDPTx, TestTypeTCPTx:
		return minThroughput * up, targetThroughput * up, nil
	case TestTypeUDPRx, TestTypeTCPRx:
		return minThroughput * down, targetThroughput * down, nil
	default:
		return 0, 0, errors.Errorf("unable to determine expected throughput, unknown test type: %s", testType)
	}
}

// RunOnce runs an Iperf session with the current configuration and returns the result.
//
// The callbox simulation should already be started before calling RunOnce.
func (c *TestManager) RunOnce(ctx context.Context, testType TestType, interfaceName string, additionalOptions []iperf.ConfigOption) (*iperf.History, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*commandTimeoutMargin)
	defer cancel()

	ipResp, err := c.client.FetchIperfIP(ctx, &manager.FetchIperfIPRequestBody{Callbox: c.callbox})
	if err != nil {
		return nil, errors.Wrap(err, "failed ot fetch DAU IP")
	}

	interfaceIP, err := getInterfaceIP(ctx, c.conn, interfaceName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get DUT celluar interface IP")
	}

	// add default options and override with additionalOptions
	options := append([]iperf.ConfigOption{}, defaultOptions[testType]...)
	options = append(options, additionalOptions...)

	// set ip tables to force traffic through cellular interface
	ipr := ip.NewRemoteRunner(c.conn)
	ipnet := net.ParseIP(ipResp.IP)
	if err := ipr.RouteIP(ctx, interfaceName, ipnet); err != nil {
		return nil, errors.Wrap(err, "failed to configure ip route")
	}
	defer ipr.DeleteIPRoute(cleanupCtx, interfaceName, ipnet)

	var cfg *iperf.Config
	var session *iperf.Session
	if testType == TestTypeUDPTx || testType == TestTypeTCPTx {
		// Test is Tx/upload so DUT is client and callbox is server
		cfg, err = iperf.NewConfig(protocolMap[testType], interfaceIP, ipResp.IP, options...)
		client, err := iperf.NewRemoteClient(ctx, c.conn)
		if err != nil {
			return nil, errors.Wrap(err, "failed ot create Iperf client")
		}
		defer client.Close(cleanupCtx)

		server, err := NewCallboxIperfServer(c.callbox, c.client)
		if err != nil {
			return nil, errors.Wrap(err, "failed ot create Iperf server")
		}
		defer server.Close(cleanupCtx)
		session = iperf.NewSession(client, server)
	} else {
		cfg, err = iperf.NewConfig(protocolMap[testType], ipResp.IP, interfaceIP, options...)
		client, err := NewCallboxIperfClient(c.callbox, c.client)
		if err != nil {
			return nil, errors.Wrap(err, "failed ot create Iperf client")
		}
		defer client.Close(cleanupCtx)

		server, err := iperf.NewRemoteServer(ctx, c.conn)
		if err != nil {
			return nil, errors.Wrap(err, "failed ot create Iperf server")
		}
		defer server.Close(cleanupCtx)
		session = iperf.NewSession(client, server)
	}

	result, err := session.Run(ctx, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run Iperf session")
	}

	return &result, nil
}

func getInterfaceIP(ctx context.Context, conn *ssh.Conn, interfaceName string) (string, error) {
	cmd := fmt.Sprintf(`ifconfig %s | grep "inet " | awk '{print $2}'`, interfaceName)
	out, err := conn.CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to get interface IP")
	}

	outString := strings.TrimSpace(string(out))
	if outString == "" {
		return "", errors.New("unable to determine IP for interface, no address found")
	}

	return outString, nil
}
