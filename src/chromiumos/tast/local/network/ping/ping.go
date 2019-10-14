// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

const (
	pingCmd = "ping"
)

// QOSType is a enum type for ping QOS option.
type QOSType int

// Enum of QOSType.
const (
	QOSBK QOSType = 0x02
	QOSBE QOSType = 0x04
	QOSVI QOSType = 0x08
	QOSVO QOSType = 0x10
)

// Regular expessions used when parsing ping output.
var (
	packetSentRE     = regexp.MustCompile(`([0-9]+) packets transmitted`)
	packetReceivedRE = regexp.MustCompile(`([0-9]+) received`)
	packetLossRE     = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)% packet loss`)
	packetStatRE     = regexp.MustCompile(`(?:round-trip|rtt) min[^=]*= ` +
		`([0-9]+(?:\.[0-9]+)?)/([0-9]+(?:\.[0-9]+)?)/` +
		`([0-9]+(?:\.[0-9]+)?)/([0-9]+(?:\.[0-9]+)?)`)
)

// config is a struct that contains the ping command parameters.
type config struct {
	BindAddress bool
	Count       int
	Size        int
	Interval    float64
	QOS         QOSType
	SourceIface string
}

// Option is a function used to configure ping command.
type Option func(c *config)

// Result is a struct that contains a successful ping's statistics.
type Result struct {
	Sent       int
	Received   int
	Loss       float64
	MinLatency float64
	AvgLatency float64
	MaxLatency float64
	DevLatency float64
}

// BindAddress returns an Option that can be passed to Ping to disallow ping
// from changing source address.
func BindAddress(bind bool) Option {
	return func(c *config) { c.BindAddress = bind }
}

// Count returns an Option that can be passed to Ping func to set ping count.
func Count(count int) Option {
	return func(c *config) { c.Count = count }
}

// Size returns an Option that can be passed to Ping to set packet size.
func Size(size int) Option {
	return func(c *config) { c.Size = size }
}

// Interval returns an Option that can be passed to Ping to set interval (in seconds).
func Interval(interval float64) Option {
	return func(c *config) { c.Interval = interval }
}

// QOS returns an Option that can be passed to Ping to set QOS type.
func QOS(qos QOSType) Option {
	return func(c *config) { c.QOS = qos }
}

// SourceIface returns an Option that can be passed to Ping to set source interface.
func SourceIface(iface string) Option {
	return func(c *config) { c.SourceIface = iface }
}

// Ping performs a shell ping with parameters specified in Options.
// If no Option is specified, default config (count=3, interval=0.5s) is used.
// Notice that when no reply is received, this function will return error as
// ping exits with code=1 in this case.
func Ping(ctx context.Context, targetIP string, options ...Option) (*Result, error) {
	cfg := &config{Count: 3, Interval: 0.5}
	for _, opt := range options {
		opt(cfg)
	}
	args, err := cfgToArgs(targetIP, cfg)
	if err != nil {
		return nil, err
	}
	output, err := testexec.CommandContext(ctx, pingCmd, args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "ping command failed")
	} else if len(output) == 0 {
		return nil, errors.New("ping returns empty stdout")
	}
	return parseOutput(string(output))
}

// cfgToArgs converts a config into a string of arguments for the ping command.
func cfgToArgs(targetIP string, cfg *config) ([]string, error) {
	var args []string
	if cfg.BindAddress {
		args = append(args, "-B")
	}
	args = append(args, "-c", strconv.Itoa(cfg.Count))
	if cfg.Size != 0 {
		args = append(args, "-s", strconv.Itoa(cfg.Size))
	}
	if cfg.Interval != 0 {
		args = append(args, "-i", fmt.Sprintf("%f", cfg.Interval))
	}
	if cfg.SourceIface != "" {
		args = append(args, "-I", cfg.SourceIface)
	}
	if cfg.QOS != 0 {
		args = append(args, "-Q", fmt.Sprintf("0x%x", cfg.QOS))
	}
	args = append(args, targetIP)
	return args, nil
}

// parseOutput parses the output of `ping` commands into a single Result.
func parseOutput(out string) (*Result, error) {
	m := packetSentRE.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.New("Parse error on sent packets")
	}
	sent, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, err
	}

	m = packetReceivedRE.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.Errorf("Parse error on received packets: %s", out)
	}
	rcv, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, err
	}

	// Helper function to parse float.
	parseFloat := func(str string) float64 {
		out, err2 := strconv.ParseFloat(str, 64)
		if err2 != nil && err == nil {
			err = err2
			return 0
		}
		return out
	}

	m = packetLossRE.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.Errorf("Parse error on lost packets. matched groups : %d", len(m))
	}
	loss := parseFloat(m[1])
	m = packetStatRE.FindStringSubmatch(out)
	if len(m) != 5 {
		return nil, errors.New("Parse error on latency statistics")
	}
	ret := &Result{
		Sent:       sent,
		Received:   rcv,
		Loss:       loss,
		MinLatency: parseFloat(m[1]),
		AvgLatency: parseFloat(m[2]),
		MaxLatency: parseFloat(m[3]),
		DevLatency: parseFloat(m[4]),
	}
	// Check if any error in helper.
	if err != nil {
		return nil, err
	}
	return ret, nil
}
