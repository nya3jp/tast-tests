// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arping contains utility functions to wrap around the arping program.
package arping

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
)

const arpingCmd = "arping"

// Result contains the sorted out output of arping command.
type Result struct {
	Sent          int
	Received      int
	Loss          float64
	AvgLatency    float64
	ResponderIPs  []string
	ResponderMACs []string
	Latencies     []float64
}

// config contains the arping command parameters.
type config struct {
	timeout float64
	count   int
}

// Option is a function to configure arping command.
type Option func(*config)

// Runner is the object contains arping utilities.
type Runner struct {
	cmd cmd.Runner
}

// NewRunner creates a new arping command utility runner.
func NewRunner(c cmd.Runner) *Runner {
	return &Runner{cmd: c}
}

// Arping performs a arping from the specified interface to the target IP with the options.
// By default 10 packets will be sent and the timeout will be the same as the count in seconds.
// It sends only broadcast ARPs.
func (r *Runner) Arping(ctx context.Context, targetIP, iface string, ops ...Option) (*Result, error) {
	conf := &config{count: 10}
	for _, op := range ops {
		op(conf)
	}
	if conf.timeout == 0 {
		conf.timeout = float64(conf.count)
	}

	args := []string{"-b"} // Default to send only broadcast ARPs.
	args = append(args, "-w", fmt.Sprint(conf.timeout))
	args = append(args, "-c", strconv.Itoa(conf.count))
	args = append(args, "-I", iface)
	args = append(args, targetIP)

	output, err := r.cmd.Output(ctx, arpingCmd, args...)

	// arping will return non-zero value when no reply received. It would
	// be convenient if the caller can distinguish the case from command
	// error. Always try to parse the output here.
	res, parseErr := parseOutput(string(output))
	if parseErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, parseErr
	}
	return res, nil
}

// Count returns an option that sets the packets count arping should send.
func Count(count int) Option {
	return func(c *config) { c.count = count }
}

// Timeout returns an option that sets timeout of arping in seconds.
func Timeout(timeout float64) Option {
	return func(c *config) { c.timeout = timeout }
}

func parseOutput(out string) (*Result, error) {
	matches := sentRE.FindStringSubmatch(out)
	if len(matches) != 2 {
		return nil, errors.Errorf("failed to parse sent probes, matches=%v", matches)
	}
	sent, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse sent")
	}

	matches = receivedRE.FindStringSubmatch(out)
	if len(matches) != 2 {
		return nil, errors.Errorf("failed to parse received responses, matches=%v", matches)
	}
	// The received here may include other unicasts we received during arpinging (i.e., not a "reply"),
	// which will be deducted at several lines below.
	received, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse received")
	}

	unicastMatches := unicastRE.FindAllStringSubmatch(out, -1)
	if len(unicastMatches) != received {
		return nil, errors.Errorf("unicast count not match, got %d want %d", len(unicastMatches), received)
	}
	var ips, macs []string
	var latencies []float64
	for _, m := range unicastMatches {
		if len(m) != 5 {
			return nil, errors.Errorf("regex error, got %d matches, want 5", len(m))
		}
		switch m[1] {
		case "reply":
			ips = append(ips, m[2])
			macs = append(macs, m[3])
			l, err := strconv.ParseFloat(m[4], 64)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse latency")
			}
			latencies = append(latencies, l)
		case "request":
			// We don't really care about the requests. It's only used for counting the correct received number.
			received--
		default:
			return nil, errors.Errorf("regex error, got %q, want either \"reply\" or \"request\"", m[1])
		}
	}

	var loss float64
	if sent != 0 {
		loss = 100 * float64(sent-received) / float64(sent)
	}
	var avgLatency float64
	if received != 0 {
		totalLatency := 0.0
		for _, l := range latencies {
			totalLatency += l
		}
		avgLatency = totalLatency / float64(received)
	}

	return &Result{
		Sent:          sent,
		Received:      received,
		Loss:          loss,
		AvgLatency:    avgLatency,
		ResponderIPs:  ips,
		ResponderMACs: macs,
		Latencies:     latencies,
	}, nil
}
