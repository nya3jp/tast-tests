// Copyright 2010 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Config is a struct that contains the ping command parameters.
type Config struct {
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
// Ping timeouts can be specified in the ctx parameter.
func Ping(ctx context.Context, cfg Config) (*Result, error) {
	res, err := cfgToArgs(cfg)
	if err != nil {
		return nil, err
	}
	var pingres *Result
	ctx, cancel := context.WithTimeout(ctx, 1000*time.Millisecond)
	defer cancel() // releases resources if slowOperation completes before timeout elapses
	o, err := testexec.CommandContext(ctx, "ping", res...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "ping failed")
	} else if string(o) == "" {
		return nil, errors.New("empty stdout")
	}
	pingres, err = parseOutput(string(o))
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return pingres, nil
}

// SimplePing pings a destination address from the DUT with default parameters.
// Ping timeouts can be set within ctx.
func SimplePing(ctx context.Context, hostName string) (bool, error) {
	cfg := Config{TargetIP: hostName, Count: 1, Interval: 1, SourceIface: "wlan0"}
	res, err := Ping(ctx, cfg)
	return res != nil && res.Received != 0, err
}

// cfgToArgs converts a Config into a string of arguments for the ping command.
func cfgToArgs(cfg Config) ([]string, error) {
	args := []string{"-B"}
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
			return []string{}, errors.Errorf("Unknown QOS value: %s", cfg.QOS)
		}
	}
	args = append(args, cfg.TargetIP)
	return args, nil
}

// getFloat is a helper that converts a string to a float and ignores errors.
// This is only used in the context of extracting matched group information
// from regex outputs as that is guaranteed to be able to be turned into a float.
func getFloat(str string) float64 {
	out, _ := strconv.ParseFloat(str, 64)
	return out
}

// parseOutput parses the output of `ping` commands into a single Result.
func parseOutput(out string) (*Result, error) {
	r := regexp.MustCompile(`([0-9]+) packets transmitted`)
	m := r.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.New("Parse error on sent packets")
	}
	sent, _ := strconv.Atoi(m[1])

	r = regexp.MustCompile(`([0-9]+) received`)
	m = r.FindStringSubmatch(out)
	if len(m) != 2 {
		return nil, errors.Errorf("Parse error on received packets: %s", out)
	}
	rcv, _ := strconv.Atoi(m[1])

	r = regexp.MustCompile(`([0-9]+(\.[0-9]+)?)% packet loss`)
	m = r.FindStringSubmatch(out)
	if len(m) != 3 {
		return nil, errors.New(fmt.Sprintf("Parse error on "+
			"lost packets. matched groups : %d", len(m)))
	}
	loss := getFloat(m[1])
	r = regexp.MustCompile(`(round-trip|rtt) min[^=]*=` +
		` ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+)`)
	m = r.FindStringSubmatch(out)
	if len(m) == 1 {
		return &Result{Sent: sent, Received: rcv, Loss: loss}, nil
	}
	if len(m) != 6 {
		return nil, errors.New("Parse error on latency statistics")
	}
	return &Result{
		Sent:       sent,
		Received:   rcv,
		Loss:       loss,
		MinLatency: getFloat(m[2]),
		AvgLatency: getFloat(m[3]),
		MaxLatency: getFloat(m[4]),
		DevLatency: getFloat(m[5]),
	}, nil
}
