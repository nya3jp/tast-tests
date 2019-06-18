// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type pingConfig struct {
	TargetIP    string
	Count       int
	Size        int
	Interval    float64
	QOS         string
	SourceIface string
}

type pingResult struct {
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
func Ping(ctx context.Context, s *testing.State, cfg pingConfig) (pingResult, error) {
	cmd := []string{"ping"}
	res, err := cfgToArgs(cfg)
	if err != nil {
		return pingResult{}, err
	}
	cmd = append(cmd, res...)
	out, err := run(ctx, strings.Trim(strings.Join(cmd, " "), "[]"))
	if status, _ := testexec.GetWaitStatus(err); int(status) != 0 {
		return pingResult{}, errors.New(fmt.Sprintf("Ping timed out, can't"+
			" parse output. Exit status: %d", status))
	}
	if out == "" {
		return pingResult{}, errors.New(fmt.Sprintf("No output from stdout."+
			" stderr is %s", err.Error()))
	}
	result, err := parseOutput(out)
	if err != nil {
		return pingResult{}, err
	}
	return result, nil
}

// SimplePing pings a destination address from the DUT with default parameters.
// Ping timeouts can be set within ctx.
func SimplePing(ctx context.Context, s *testing.State,
	hostName string) (bool, error) {
	cfg := NewPingConfig(hostName)
	cfg.Count = 3
	cfg.Interval = 10
	res, err := Ping(ctx, s, cfg)
	return res != pingResult{} && res.Received != 0, err
}

// NewPingConfig is a factory to create a default pingConfig with limited args.
func NewPingConfig(ip string) pingConfig {
	return pingConfig{TargetIP: ip,
		Count:       10,
		Size:        0,
		Interval:    0,
		QOS:         "",
		SourceIface: ""}
}

// NewFullPingConfig is a factory to create a pingConfig with all arguments specified..
func NewFullPingConfig(ip string, ct int, size int, interval float64,
	iface string, qos string) pingConfig {
	return pingConfig{TargetIP: ip,
		Count:       ct,
		Size:        size,
		Interval:    interval,
		SourceIface: iface,
		QOS:         qos,
	}
}

// NewPingResult is a factory that creates a pingResult with limited parameters.
func NewPingResult(snt int, rcv int, loss float64) pingResult {
	return pingResult{Sent: snt, Received: rcv, Loss: loss}
}

// NewFullPingResult is a factory that creates a pingResult with more parameters
// than NewPingResult.
func NewFullPingResult(snt int, rcv int, loss float64,
	minL float64, avgL float64, maxL float64, devL float64) pingResult {
	return pingResult{
		Sent:       snt,
		Received:   rcv,
		Loss:       loss,
		MinLatency: minL,
		AvgLatency: avgL,
		MaxLatency: maxL,
		DevLatency: devL,
	}
}

// cfgToArgs converts a pingConfig into a string of arguments for the ping command.
func cfgToArgs(cfg pingConfig) ([]string, error) {
	args := []string{}
	args = append(args, fmt.Sprintf("-c %d", cfg.Count))
	if cfg.Size != 0 {
		args = append(args, fmt.Sprintf("-s %d", cfg.Size))
	}
	if cfg.Interval != 0 {
		args = append(args, fmt.Sprintf("-i %f", cfg.Interval))
	}
	if cfg.SourceIface != "" {
		args = append(args, fmt.Sprintf("-I %s", cfg.SourceIface))
	}
	if cfg.QOS != "" {
		switch cfg.QOS {
		case "be":
			args = append(args, "-Q 0x04")
		case "bk":
			args = append(args, "-Q 0x02")
		case "vi":
			args = append(args, "-Q 0x08")
		case "vo":
			args = append(args, "-Q 0x10")
		default:
			return []string{}, errors.New(fmt.Sprintf("Unknown QOS value: %s", cfg.QOS))
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

// parseOutput parses the output of `ping` commands into a single pingResult.
func parseOutput(out string) (pingResult, error) {
	sentMatch := regexp.MustCompile(`([0-9]+) packets transmitted`)
	matchGroup := sentMatch.FindStringSubmatch(out)
	if len(matchGroup) != 2 {
		return pingResult{}, errors.New("Parse error on sent packets")
	}
	sent, _ := strconv.Atoi(matchGroup[1])

	rcvMatch := regexp.MustCompile(`([0-9]+) packets received`)
	matchGroup = rcvMatch.FindStringSubmatch(out)
	if len(matchGroup) != 2 {
		return pingResult{}, errors.New("Parse error on received packets")
	}
	rcv, _ := strconv.Atoi(matchGroup[1])

	lossMatch := regexp.MustCompile(`([0-9]+(\.[0-9]+)?)% packet loss`)
	matchGroup = lossMatch.FindStringSubmatch(out)
	if len(matchGroup) != 3 {
		return pingResult{}, errors.New(fmt.Sprintf("Parse error on "+
			"lost packets. matched groups : %d", len(matchGroup)))
	}
	loss := getFloat(matchGroup[1])
	latMatch := regexp.MustCompile(`(round-trip|rtt) min[^=]*=` +
		` ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+)`)
	matchGroup = latMatch.FindStringSubmatch(out)
	if len(matchGroup) == 1 {
		return NewPingResult(sent, rcv, loss), nil
	}
	if len(matchGroup) != 6 {
		return pingResult{}, errors.New("Parse error on latency statistics")
	}
	return NewFullPingResult(sent, rcv, loss, getFloat(matchGroup[2]),
		getFloat(matchGroup[3]), getFloat(matchGroup[4]),
		getFloat(matchGroup[5])), nil
}

// run executes a shell command on the DUT and returns its output.
// run executes in a blocking fashion and will not return until the
// command terminates.
func run(ctx context.Context, shellCommand string) (string, error) {
	out, err := testexec.CommandContext(ctx, "sh", "-c", shellCommand).Output()
	return string(out), err
}
