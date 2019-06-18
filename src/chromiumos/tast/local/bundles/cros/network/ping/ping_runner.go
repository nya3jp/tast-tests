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

func NewPingConfig(ip string) pingConfig {
	return pingConfig{TargetIP: ip,
		Count:       10,
		Size:        0,
		Interval:    0,
		QOS:         "",
		SourceIface: ""}
}

func NewPingResult(snt int, rcv int, loss rcv) pingResult {
	return pingResult{Sent: snt, Received: rcv, Loss: loss}
}

func NewFullPingResult(snt int, rcv int, loss int, minL float64, avgL float64, maxL float64, devL float64) pingResult {
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

func cfgToArgs() (string, error) {
	args := []string{}
	args = append(args, fmt.Sprintf("-c %d", pr.Config.Count))
	if pr.Config.Size != 0 {
		args = append(args, fmt.Sprintf("-s %d", pr.Config.Size))
	}
	if pr.Config.Interval != 0 {
		args = append(args, fmt.Sprintf("-i %f", pr.Config.Interval))
	}
	if pr.Config.Interval != 0 {
		args = append(args, fmt.Sprintf("-i %f", pr.Config.Interval))
	}
	if pr.Config.SourceIface != "" {
		args = append(args, fmt.Sprintf("-i %s", pr.Config.SourceIface))
	}
	if pr.Config.Interval != 0 {
		args = append(args, fmt.Sprintf("-i %f", pr.Config.Interval))
	}
	if pr.Config.QOS != "" {
		switch pr.Config.QOS {
		case "be":
			args = append(args, "-Q 0x04")
		case "bk":
			args = append(args, "-Q 0x02")
		case "vi":
			args = append(args, "-Q 0x08")
		case "vo":
			args = append(args, "-Q 0x10")
		default:
			return nil, errors.New(fmt.Sprintf("Unknown QOS value: %s", pr.Config.QOS))
		}
	}
	args = append(args, pr.Config.TargetIP)
	return args, nil
}

func getFloat(str string) float64 {
	out, _ := strconv.ParseFloat(str, 64)
	return out
}

func parseOutput(out string) (pingResult, error) {
	sentMatch := regexp.MustCompile(`([0-9]+) packets transmitted`)
	matchGroup := sentMatch.FindStringSubmatch(out)
	if len(matchGroup) != 2 {
		return nil, errors.New("Parse error on sent packets")
	}
	sent, _ := strconv.Atoi(matchGroup[1])

	rcvMatch := regexp.MustCompile(`([0-9]+) packets received`)
	matchGroup = rcvMatch.FindStringSubmatch(out)
	if len(matchGroup) != 2 {
		return nil, errors.New("Parse error on received packets")
	}
	rcv, _ := strconv.Atoi(matchGroup[1])

	lossMatch := regexp.MustCompile(`([0-9]+(\.[0-9]+)?)% packet loss`)
	matchGroup = lossMatch.FindStringSubmatch(out)
	if len(matchGroup) != 2 {
		return nil, errors.New("Parse error on lost packets")
	}
	loss, _ := strconv.Atoi(matchGroup[1])
	latMatch := regexp.MustCompile(`(round-trip|rtt) min[^=]*= ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+)`)
	matchGroup = latMatch.FindStringSubmatch(out)
	if len(matchGroup) == 1 {
		return NewPingResult(sent, rcv, loss), nil
	}
	if len(matchGroup) != 6 {
		return nil, errors.New("Parse error on latency statistics")
	}
	return NewFullPingResult(sent, rcv, loss, getFloat(matchGroup[2]), getFloat(matchGroup[3]), getFloat(matchGroup[4]), getFloat(matchGroup[5])), nil
}

func simplePing(ctx context.Context, s *testing.State, hostName string) (bool, err) {
	pingConfig = NewPingConfig(hostName, 3, .5)
	pingResult, err = ping(ctx, s, pingConfig)
	return pingResult != nil && pingResult.received != 0, err
}

func run(ctx context.Context, shellCommand string) (string, error) {
	out, err := testexec.CommandContext(ctx, "sh", "-c", shellCommand).Output()
	return string(out), err
}
func ping(ctx context.Context, s *testing.State, cfg pingConfig) (pingResult, error) {
	cmd := []string{"ping"}
	cmd = append(cmd, cfgToArgs(cfg))
	out, err := run(ctx, cmd)
	if status, _ := testexec.GetWaitStatus(err); int(status) != 0 {
		return nil, errors.New(fmt.Sprintf("Ping timed out, can't parse output.", status))
	}
	if out == "" {
		return nil, errors.New(fmt.Sprintf("No output from stdout. stderr is %s", err.Error()))
	}
	result, err := parseOutput(out)
	if err != nil {
		return nil, err
	}
	return result, nil
}
