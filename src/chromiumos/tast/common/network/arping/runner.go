// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arping contains utility functions to wrap around the arping program.
package arping

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const arpingCmd = "arping"

// Result contains the sorted out output of arping command.
type Result struct {
	Sent          int
	Received      int
	Loss          float64
	AvgLatency    time.Duration
	ResponderIPs  []string
	ResponderMACs []string
	Latencies     []time.Duration
}

// String returns a overview of the Result.
func (r *Result) String() string {
	return fmt.Sprintf("send=%d received=%d loss=%g avgLatency=%v", r.Sent, r.Received, r.Loss, r.AvgLatency)
}

// config contains the arping command parameters.
type config struct {
	timeout time.Duration
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

// Arping performs an arping from the specified interface to the target IP with the options.
// By default 10 packets will be sent and the timeout will be the same as the count in seconds.
// It sends only broadcast ARPs.
func (r *Runner) Arping(ctx context.Context, targetIP, iface string, ops ...Option) (*Result, error) {
	conf := &config{count: 10}
	for _, op := range ops {
		op(conf)
	}
	if conf.timeout == 0 {
		conf.timeout = time.Duration(conf.count) * time.Second
	}

	timeout := conf.timeout.Truncate(time.Second)
	if timeout != conf.timeout {
		testing.ContextLogf(ctx, "arping timeout accepts only integer in seconds, truncated from %v to %v", conf.timeout, timeout)
	}

	args := []string{"-b"} // Default to send only broadcast ARPs.
	args = append(args, "-w", strconv.Itoa(int(timeout.Seconds())))
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
		// Log the output of arping if parseErr != nil and err == nil,
		// since CmdRunner should already dump when err != nil.
		if err := logArpingResult(ctx, output); err != nil {
			testing.ContextLogf(ctx, "failed to log arping: %v, output=%q", err, output)
		}
		return nil, parseErr
	}
	return res, nil
}

// logArpingResult save the output of arping to OutDir with random generated name.
func logArpingResult(ctx context.Context, output []byte) error {
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get OutDir")
	}

	f, err := ioutil.TempFile(outdir, "arping_*.log")
	if err != nil {
		return errors.Wrap(err, "failed to create log file for arping")
	}
	defer f.Close()

	testing.ContextLogf(ctx, "logging arping output to %q", filepath.Base(f.Name()))
	if _, err = f.Write(output); err != nil {
		return errors.Wrap(err, "failed to write arping output to disk")
	}
	return nil
}

// Count returns an option that sets the packets count arping should send.
func Count(count int) Option {
	return func(c *config) { c.count = count }
}

// Timeout returns an option that sets timeout of arping in seconds.
// Note that arping accepts only integer in seconds, so this option will truncate the parameter to second.
func Timeout(timeout time.Duration) Option {
	return func(c *config) { c.timeout = timeout }
}

var (
	// unicastRE regexp for unicast reply/request.
	unicastRE = regexp.MustCompile(`(?m)^Unicast (reply|request) from ` +
		`(\d{1,3}(?:\.\d{1,3}){3}) \[([0-9A-F]{2}(?::[0-9A-F]{2}){5})\]  (\d+(?:\.\d+)?ms)$`)
	// sentRE regexp for sent probes.
	sentRE = regexp.MustCompile(`(?m)^Sent (\d+) probes`)
	// receivedRE regexp for received responses.
	receivedRE = regexp.MustCompile(`(?m)^Received (\d+) response\(s\)`)
)

func parseOutput(out string) (*Result, error) {
	matches := sentRE.FindStringSubmatch(out)
	if len(matches) != 2 {
		return nil, errors.Errorf("failed to parse sent probes, matches=%v", matches)
	}
	sent, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert %q to integer while parsing sent", matches[1])
	}

	matches = receivedRE.FindStringSubmatch(out)
	if len(matches) != 2 {
		return nil, errors.Errorf("failed to parse received responses, matches=%v", matches)
	}
	// The received here may include other unicasts we received during arpinging (i.e., not a "reply"),
	// which will be deducted at several lines below.
	received, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert %q to integer while parsing received", matches[1])
	}

	unicastMatches := unicastRE.FindAllStringSubmatch(out, -1)
	if len(unicastMatches) != received {
		return nil, errors.Errorf("unicast count not match, got %d want %d", len(unicastMatches), received)
	}
	var ips, macs []string
	var latencies []time.Duration
	for _, m := range unicastMatches {
		if len(m) != 5 {
			return nil, errors.Errorf("failed to parse unicast message; got %d submatches, want 5", len(m))
		}
		switch m[1] {
		case "reply":
			ips = append(ips, m[2])
			macs = append(macs, m[3])
			l, err := time.ParseDuration(m[4])
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse latency")
			}
			latencies = append(latencies, l)
		case "request":
			// We don't really care about the requests. It's only used for counting the correct received number.
			received--
		default:
			return nil, errors.Errorf("failed to parse unicast message; got %q, want either \"reply\" or \"request\"", m[1])
		}
	}

	if sent == 0 {
		return nil, errors.New("no packet was sent")
	}

	var loss float64
	loss = 100 * float64(sent-received) / float64(sent)

	var avgLatency time.Duration
	if len(latencies) != 0 {
		for _, l := range latencies {
			avgLatency += l
		}
		avgLatency /= time.Duration(len(latencies))
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
