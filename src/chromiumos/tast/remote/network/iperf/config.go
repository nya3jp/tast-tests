// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"time"

	"chromiumos/tast/errors"
)

// TestType is the type of Iperf test to run.
type TestType int

const (
	// TestTypeTCP is an Iperf TCP test.
	TestTypeTCP TestType = iota
	// TestTypeTCPBidirectional is an Iperf bidirectional TCP test.
	TestTypeTCPBidirectional
	// TestTypeUDP is an Iperf UDP test
	TestTypeUDP
	// TestTypeUDPBidirectional is an Iperf bidirectional UDP test.
	TestTypeUDPBidirectional
)

const (
	defaultTestTime     = 10 * time.Second
	defaultMaxBandwidth = "1000M"
	defaultPort         = 5001
	defaultPortCount    = 4
	defaultWindowSize   = "320k"
)

// Config represents the configuration options for an iperf run.
type Config struct {
	TestType     TestType
	TestTime     time.Duration
	MaxBandwidth string
	WindowSize   string
	Port         int
	PortCount    int
	ClientIP     string
	ServerIP     string
}

// ConfigOption represents a configuration option to be used in an Iperf run.
type ConfigOption func(config *Config) error

// NewConfig returns an Iperf configuration for a run with the specified options.
func NewConfig(testType TestType, clientIP, serverIP string, opts ...ConfigOption) (*Config, error) {
	res := &Config{
		TestType:     testType,
		ClientIP:     clientIP,
		ServerIP:     serverIP,
		TestTime:     defaultTestTime,
		MaxBandwidth: defaultMaxBandwidth,
		WindowSize:   defaultWindowSize,
		Port:         defaultPort,
		PortCount:    defaultPortCount,
	}

	for _, opt := range opts {
		err := opt(res)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set Iperf configuration option")
		}
	}

	return res, nil
}

// TestTimeOption sets the test time to use in the Iperf run.
func TestTimeOption(time time.Duration) ConfigOption {
	return func(config *Config) error {
		config.TestTime = time
		return nil
	}
}

// MaxBandwidthOption sets the maximum bandwidth to use in the Iperf run.
func MaxBandwidthOption(maxBandwidth string) ConfigOption {
	return func(config *Config) error {
		config.MaxBandwidth = maxBandwidth
		return nil
	}
}

// WindowSizeOption sets the size of the window to use in the Iperf run.
func WindowSizeOption(windowSize string) ConfigOption {
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

func (c *Config) isBidirectional() bool {
	return c.TestType == TestTypeTCPBidirectional ||
		c.TestType == TestTypeUDPBidirectional
}

func (c *Config) isUDP() bool {
	return c.TestType == TestTypeUDP ||
		c.TestType == TestTypeUDPBidirectional
}
