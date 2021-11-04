// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
)

// ParseVMStat returns a map of all the values parsed from a /proc/vmstat file.
func ParseVMStat(vmstat string) (map[string]uint64, error) {
	m := make(map[string]uint64)
	// NB: Use FieldsFunc instead of Split to skip empty lines.
	lines := strings.FieldsFunc(vmstat, func(r rune) bool { return r == '\n' })
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, errors.Errorf("unexpected line in /proc/vmstat, expecting two fields in %q", line)
		}
		name := fields[0]
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse value from vmstat line %q", line)
		}
		m[name] = value
	}
	return m, nil
}

// VMStatMetrics writes the contents of `/proc.vmstat` to outdir. If outdir is
// "", then no logs are written. If p is provided, it adds the following
// metrics:
//  - arcvm_virtio_balloon - The size of the virtio_balloon, in bytes.
func VMStatMetrics(ctx context.Context, a *arc.ARC, p *perf.Values, outdir, suffix string) error {
	vmstat, err := a.Command(ctx, "cat", "/proc/vmstat").Output()
	if err != nil {
		return errors.Wrap(err, "failed to cat /proc/vmstat")
	}

	// Keep a copy in logs for debugging.
	if len(outdir) > 0 {
		outfile := "arc.vmstat" + suffix + ".txt"
		outpath := filepath.Join(outdir, outfile)
		if err := ioutil.WriteFile(outpath, vmstat, 0644); err != nil {
			return errors.Wrapf(err, "failed to write vmstat to %q", outpath)
		}
	}

	if p == nil {
		// No perf.Values, so don't compute metrics.
		return nil
	}

	m, err := ParseVMStat(string(vmstat))
	if err != nil {
		return errors.Wrap(err, "failed to parse vmstat file from ARC")
	}
	inflate, ok := m["balloon_inflate"]
	if !ok {
		return errors.New("arcvm /proc/vmstat missing balloon_inflate")
	}
	deflate, ok := m["balloon_deflate"]
	if !ok {
		return errors.New("arcvm /proc/vmstat missing balloon_deflate")
	}

	p.Set(
		perf.Metric{
			Name: fmt.Sprintf("arcvm_virtio_balloon%s", suffix),
			Unit: "MiB",
		},
		float64(inflate-deflate)/256.0, // NB: there are 256 pages per MiB.
	)

	return nil
}
