// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
)

const (
	pingCmd                   = "ping"
	pingLossThreshold float64 = 20 // A percentage.
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

// config is a struct that contains the ping command parameters.
type config struct {
	bindAddress  bool
	count        int
	size         int
	interval     float64
	qos          QOSType
	sourceIface  string
	user         string
	ignoreResult bool
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

// Runner is the object contains ping utilities.
type Runner struct {
	cmd cmd.Runner
}

// NewRunner creates a new ping command utility runner.
func NewRunner(c cmd.Runner) *Runner {
	return &Runner{cmd: c}
}

// Ping performs a shell ping with parameters specified in Options.
// If no Option is specified, default config (count=3, interval=0.5s) is used.
// Notice that when no reply is received, this function will try to parse the
// output and return a valid result instead of returning the error of non-zero
// return code of ping.
func (r *Runner) Ping(ctx context.Context, targetIP string, options ...Option) (*Result, error) {
	cfg := &config{count: 10, interval: 0.5}
	for _, opt := range options {
		opt(cfg)
	}
	args, err := cfg.cmdArgs(targetIP)
	if err != nil {
		return nil, err
	}

	command := pingCmd
	if cfg.user != "" {
		command = "su"
		userCmd := shutil.EscapeSlice(append([]string{pingCmd}, args...))
		args = []string{cfg.user, "-c", userCmd}
	}

	output, err := r.cmd.Output(ctx, command, args...)

	// ping will return non-zero value when no reply received. It would
	// be convenient if the caller can distinguish the case from command
	// error. Always try to parse the output here.
	res, parseErr := parseOutput(string(output))
	if parseErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, parseErr
	}

	if res.Loss > pingLossThreshold && !cfg.ignoreResult {
		return nil, errors.Errorf("unexpectd packet loss percentage: got %g%%, want < %g%%", res.Loss, pingLossThreshold)
	}

	return res, nil
}

// BindAddress returns an Option that can be passed to Ping to disallow ping
// from changing source address.
func BindAddress(bind bool) Option {
	return func(c *config) { c.bindAddress = bind }
}

// Count returns an Option that can be passed to Ping func to set ping count.
func Count(count int) Option {
	return func(c *config) { c.count = count }
}

// Size returns an Option that can be passed to Ping to set packet size.
func Size(size int) Option {
	return func(c *config) { c.size = size }
}

// Interval returns an Option that can be passed to Ping to set interval (in seconds).
func Interval(interval float64) Option {
	return func(c *config) { c.interval = interval }
}

// QOS returns an Option that can be passed to Ping to set QOS type.
func QOS(qos QOSType) Option {
	return func(c *config) { c.qos = qos }
}

// SourceIface returns an Option that can be passed to Ping to set source interface.
func SourceIface(iface string) Option {
	return func(c *config) { c.sourceIface = iface }
}

// User returns an Option that can be passed to Ping to set user.
func User(user string) Option {
	return func(c *config) { c.user = user }
}

// IgnoreResult returns an Option that can be passed to Ping to ignore the result.
func IgnoreResult(ignore bool) Option {
	return func(c *config) { c.ignoreResult = ignore }
}

// cmdArgs converts a config into a string of arguments for the ping command.
func (cfg *config) cmdArgs(targetIP string) ([]string, error) {
	var args []string
	if cfg.bindAddress {
		args = append(args, "-B")
	}
	args = append(args, "-c", strconv.Itoa(cfg.count))
	if cfg.size != 0 {
		args = append(args, "-s", strconv.Itoa(cfg.size))
	}
	if cfg.interval != 0 {
		args = append(args, "-i", fmt.Sprintf("%f", cfg.interval))
	}
	if cfg.sourceIface != "" {
		args = append(args, "-I", cfg.sourceIface)
	}
	if cfg.qos != 0 {
		args = append(args, "-Q", fmt.Sprintf("0x%x", cfg.qos))
	}
	args = append(args, targetIP)
	return args, nil
}

// parseOutput parses the output of `ping` commands into a single Result.
func parseOutput(out string) (*Result, error) {
	m := sentRE.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.New("parse error on sent packets")
	}
	sent, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, err
	}

	m = receivedRE.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.Errorf("parse error on received packets: %s", out)
	}
	recv, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, err
	}

	m = lossRE.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.Errorf("parse error on lost packets. matched groups : %d", len(m))
	}
	loss, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse loss=%q to float", m[1])
	}

	if recv == 0 {
		// No received reply to have statistics, early return.
		return &Result{
			Sent:     sent,
			Received: recv,
			Loss:     loss,
		}, nil
	}

	m = statRE.FindStringSubmatch(out)
	if len(m) != 5 {
		return nil, errors.New("parse error on latency statistics")
	}
	var stats [4]float64
	for i, str := range m[1:] {
		stats[i], err = strconv.ParseFloat(str, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse stat=%q to float", str)
		}
	}

	return &Result{
		Sent:       sent,
		Received:   recv,
		Loss:       loss,
		MinLatency: stats[0],
		AvgLatency: stats[1],
		MaxLatency: stats[2],
		DevLatency: stats[3],
	}, nil
}
