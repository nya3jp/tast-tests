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
	defaultPingCmd = "ping"
)

// Config is a struct that contains the ping command parameters.
type Config struct {
	BindAddress bool
	TargetIP    string
	Count       int
	Size        int
	Interval    float64
	QOS         string
	SourceIface string
}

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

// Ping performs a shell ping with parameters specified in a ping config.
// Notice that when no reply is received, this function will return error
// as ping exits with code=1 in this case.
func Ping(ctx context.Context, cfg Config) (*Result, error) {
	args, err := cfgToArgs(cfg)
	if err != nil {
		return nil, err
	}
	output, err := testexec.CommandContext(ctx, defaultPingCmd, args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "ping command failed")
	} else if len(output) == 0 {
		return nil, errors.New("ping returns empty stdout")
	}
	return parseOutput(string(output))
}

// SimplePing pings a destination address with default parameters.
func SimplePing(ctx context.Context, hostName string) error {
	cfg := Config{TargetIP: hostName, Count: 3, Interval: 0.5}
	_, err := Ping(ctx, cfg)
	// If no packet is received, then ping will exit with 1 which results in an error,
	// we can just return err here.
	return err
}

// cfgToArgs converts a Config into a string of arguments for the ping command.
func cfgToArgs(cfg Config) ([]string, error) {
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
	if cfg.QOS != "" {
		switch cfg.QOS {
		case "be":
			args = append(args, "-Q", "0x04")
		case "bk":
			args = append(args, "-Q", "0x02")
		case "vi":
			args = append(args, "-Q", "0x08")
		case "vo":
			args = append(args, "-Q", "0x10")
		default:
			return []string{}, errors.Errorf("unknown QOS value: %s", cfg.QOS)
		}
	}
	args = append(args, cfg.TargetIP)
	return args, nil
}

// parseFloat is a helper that converts a string to a float and panic on errors.
// This is only used in the context of extracting matched group information
// from regex outputs as that is guaranteed to be able to be turned into a float.
func parseFloat(str string) float64 {
	out, err := strconv.ParseFloat(str, 64)
	if err != nil {
		// Should never reach.
		panic(errors.Wrap(err, "float parse failed"))
	}
	return out
}

// parseOutput parses the output of `ping` commands into a single Result.
func parseOutput(out string) (*Result, error) {
	r := regexp.MustCompile(`([0-9]+) packets transmitted`)
	m := r.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.New("Parse error on sent packets")
	}
	sent, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, err
	}

	r = regexp.MustCompile(`([0-9]+) received`)
	m = r.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.Errorf("Parse error on received packets: %s", out)
	}
	rcv, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, err
	}

	r = regexp.MustCompile(`([0-9]+(\.[0-9]+)?)% packet loss`)
	m = r.FindStringSubmatch(out)
	if len(m) != 3 {
		return nil, errors.Errorf("Parse error on lost packets. matched groups : %d", len(m))
	}
	loss := parseFloat(m[1])
	r = regexp.MustCompile(`(round-trip|rtt) min[^=]*=` +
		` ([0-9]+(\.[0-9]+)?)/([0-9]+(\.[0-9]+)?)/` +
		`([0-9]+(\.[0-9]+)?)/([0-9]+(\.[0-9]+)?)`)
	m = r.FindStringSubmatch(out)
	if len(m) != 10 {
		return nil, errors.New("Parse error on latency statistics")
	}
	return &Result{
		Sent:       sent,
		Received:   rcv,
		Loss:       loss,
		MinLatency: parseFloat(m[2]),
		AvgLatency: parseFloat(m[4]),
		MaxLatency: parseFloat(m[6]),
		DevLatency: parseFloat(m[8]),
	}, nil
}
