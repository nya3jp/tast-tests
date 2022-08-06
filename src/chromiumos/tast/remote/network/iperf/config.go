// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"time"

	"chromiumos/tast/errors"
)

// Protocol represents the type of protocol to use in a test.
type Protocol string

const (
	// ProtocolTCP represents a TCP connection.
	ProtocolTCP Protocol = "TCP"
	// ProtocolUDP represents a UDP connection.
	ProtocolUDP Protocol = "UDP"
)

// BitRate represents a bit rate in bit/s.
type BitRate float64

const (
	// Gbps represents 1 Gbit/s.
	Gbps BitRate = 1000000000
	// Mbps represents 1 Mbit/s.
	Mbps = 1000000
	// Kbps represents 1 Kbit/s.
	Kbps = 1000
	// Bps represents 1 bit/s.
	Bps = 1
)

// ByteSize represents a number of Bytes in iperf.
type ByteSize int64

const (
	// GB represents 1 Gigabyte.
	GB ByteSize = 1000000000
	// MB represents 1 Megabyte.
	MB = 1000000
	// KB represents 1 Kilobyte.
	KB = 1000
	// B represents 1 Byte.
	B = 1
)

const (
	defaultTestTime      = 10 * time.Second
	defaultMaxBandwidth  = 1 * Gbps
	defaultPort          = 5001
	defaultPortCount     = 4
	defaultWindowSize    = 320 * KB
	defaultBidirectional = false
)

// Config represents the configuration options for an iperf run.
type Config struct {
	Protocol           Protocol
	Bidirectional      bool
	TestTime           time.Duration
	MaxBandwidth       BitRate
	WindowSize         ByteSize
	Port               int
	PortCount          int
	ClientIP           string
	ServerIP           string
	FetchServerResults bool
}

// ConfigOption represents a configuration option to be used in an Iperf run.
type ConfigOption func(config *Config) error

// NewConfig returns an Iperf configuration for a run with the specified options.
func NewConfig(protocol Protocol, clientIP, serverIP string, opts ...ConfigOption) (*Config, error) {
	res := &Config{
		Protocol:      protocol,
		ClientIP:      clientIP,
		ServerIP:      serverIP,
		Bidirectional: defaultBidirectional,
		TestTime:      defaultTestTime,
		MaxBandwidth:  defaultMaxBandwidth,
		WindowSize:    defaultWindowSize,
		Port:          defaultPort,
		PortCount:     defaultPortCount,
	}

	for _, opt := range opts {
		err := opt(res)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set Iperf configuration option")
		}
	}

	return res, nil
}

// FetchServerResultsOption sets if the test should fetch the iperf results from the server rather than the client.
func FetchServerResultsOption(useServerResults bool) ConfigOption {
	return func(config *Config) error {
		config.FetchServerResults = useServerResults
		return nil
	}
}

// BidirectionalOption sets if the test should be bidirectional.
func BidirectionalOption(bidirectional bool) ConfigOption {
	return func(config *Config) error {
		config.Bidirectional = bidirectional
		return nil
	}
}

// TestTimeOption sets the test time to use in the Iperf run.
func TestTimeOption(time time.Duration) ConfigOption {
	return func(config *Config) error {
		config.TestTime = time
		return nil
	}
}

// MaxBandwidthOption sets the maximum bandwidth to use in the Iperf run.
func MaxBandwidthOption(maxBandwidth BitRate) ConfigOption {
	return func(config *Config) error {
		config.MaxBandwidth = maxBandwidth
		return nil
	}
}

// WindowSizeOption sets the size of the window to use in the Iperf run.
func WindowSizeOption(windowSize ByteSize) ConfigOption {
	return func(config *Config) error {
		config.WindowSize = windowSize
		return nil
	}
}

// PortOption sets the port number to use in the Iperf run.
func PortOption(port int) ConfigOption {
	return func(config *Config) error {
		config.Port = port
		return nil
	}
}

// PortCountOption sets the number of parallel threads to use in the Iperf run.
func PortCountOption(portCount int) ConfigOption {
	return func(config *Config) error {
		config.PortCount = portCount
		return nil
	}
}
