// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"time"
)

// TestType defines type of tests possible to run in netperf.
type TestType string

const (
	// TestTypeTCPCRR measures how many times we can connect, request a byte,
	// and receive a byte per second.
	TestTypeTCPCRR TestType = "TCP_CRR"
	// TestTypeTCPMaerts : maerts is stream backwards. Like TCP_STREAM, except
	// it measures stream's bitrate in opposite direction: from the netperf server
	// to the client.
	TestTypeTCPMaerts = "TCP_MAERTS"
	// TestTypeTCPRR measures how many times we can request a byte and receive
	// a byte per second.
	TestTypeTCPRR = "TCP_RR"
	// TestTypeTCPSendfile is like a TCP_STREAM test except that the netperf client
	// will use a platform dependent call like sendfile() rather than the simple
	// send() call. This can result in better performance.
	TestTypeTCPSendfile = "TCP_SENDFILE"
	// TestTypeTCPStream measures throughput sending bytes from the client
	// to the server in a TCP stream.
	TestTypeTCPStream = "TCP_STREAM"
	// TestTypeUDPRR measures how many times we can request a byte from the client
	// and receive a byte from the server. If any datagram is dropped, the client
	// or server will block indefinitely. This failure is not evident except
	// as a low transaction rate.
	TestTypeUDPRR = "UDP_RR"
	// TestTypeUDPStream tests UDP throughput sending from the client to the server.
	// There is no flow control here, and generally sending is easier that receiving,
	// so there will be two types of throughput, both receiving and sending.
	TestTypeUDPStream = "UDP_STREAM"
	// TestTypeUDPMaerts isn't a real test type, but we can emulate a UDP stream
	// from the server to the DUT by running the netperf server on the DUT and the
	// client on the server and then doing a UDP_STREAM test.
	TestTypeUDPMaerts = "UDP_MAERTS"
)

// Config defines configuration for netperf run.
type Config struct {
	// TestTime how long the test should be run.
	TestTime time.Duration
	// TestType is literally this: test type.
	TestType TestType
	// Reverse: reverse client and server roles.
	Reverse bool
}

const (
	dataPort                    = 12866
	controlPort                 = 12865
	netservStartupWaitTime      = 3 * time.Second
	netperfCommandTimeoutMargin = 30 * time.Second
)

var shortTags = map[TestType]string{
	TestTypeTCPCRR:      "tcp_crr",
	TestTypeTCPMaerts:   "tcp_rx",
	TestTypeTCPRR:       "tcp_rr",
	TestTypeTCPSendfile: "tcp_stx",
	TestTypeTCPStream:   "tcp_tx",
	TestTypeUDPRR:       "udp_rr",
	TestTypeUDPStream:   "udp_tx",
	TestTypeUDPMaerts:   "udp_rx",
}

var readableTags = map[TestType]string{
	TestTypeTCPCRR:      "tcp_connect_roundtrip_rate",
	TestTypeTCPMaerts:   "tcp_downstream",
	TestTypeTCPRR:       "tcp_roundtrip_rate",
	TestTypeTCPSendfile: "tcp_upstream_sendfile",
	TestTypeTCPStream:   "tcp_upstream",
	TestTypeUDPRR:       "udp_roundtrip",
	TestTypeUDPStream:   "udp_upstream",
	TestTypeUDPMaerts:   "udp_downstream",
}

// ShortTag returns shortened tag representative to the configuration.
func (c *Config) ShortTag() string {
	return shortTags[c.TestType]
}

// HumanReadableTag returns human readable tag describing the configuration.
func (c *Config) HumanReadableTag() string {
	return readableTags[c.TestType]
}
