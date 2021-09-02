// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const vmSocketGlob = "/run/vm/vm.*/*.sock"

// BalloonAdditionalStats are statistics retreived from the guest via the
// virtio_balloon device.
type BalloonAdditionalStats struct {
	SwapIn             uint64 `json:"swap_in"`
	SwapOut            uint64 `json:"swap_out"`
	MajorFaults        uint64 `json:"major_faults"`
	MinorFaults        uint64 `json:"minor_faults"`
	FreeMemory         uint64 `json:"free_memory"`
	TotalMemory        uint64 `json:"total_memory"`
	AvailableMemory    uint64 `json:"available_memory"`
	DiskAcaches        uint64 `json:"disk_caches"`
	HugeTLBAllocations uint64 `json:"hugetlb_allocations"`
	HugeTLBFailures    uint64 `json:"hugetlb_failures"`
}

// BalloonStats is a snapshot of statistics read from the virtio_balloon device.
type BalloonStats struct {
	BalloonActual   uint64                 `json:"balloon_actual"`
	AdditionalStats BalloonAdditionalStats `json:"stats"`
}

// NewBalloonStats queries the current balloon stats from crosvm for a given
// VM socket path.
func NewBalloonStats(ctx context.Context, socketPath string) (*BalloonStats, error) {
	out, err := testexec.CommandContext(ctx, "crosvm", "balloon_stats", socketPath).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call crosvm to get balloon stats")
	}
	// NB: The actual stats are wrapped in an object with a single
	// "BalloonStats" field, which isn't useful for consumers of this API, so
	// so just declare a type inline to parse it.
	var stats struct {
		Stats BalloonStats `json:"BalloonStats"`
	}
	if err := json.Unmarshal(out, &stats); err != nil {
		return nil, errors.Wrapf(err, "failed to parse balloon stats from %q", stats)
	}
	return &stats.Stats, nil
}

// VirtioBalloonMetrics logs performance metrics for the size of the
// virtio_balloon for each running crosvm.
func VirtioBalloonMetrics(ctx context.Context, p *perf.Values, suffix string) error {
	paths, err := filepath.Glob(vmSocketGlob)
	if err != nil {
		return errors.Wrap(err, "failed to find any vm sockets")
	}
	for _, path := range paths {
		stats, err := NewBalloonStats(ctx, path)
		if err != nil {
			return errors.Wrapf(err, "failed to get balloon stats from socket %q", path)
		}
		// TODO: this name only works for arcvm, every other VM has a socket
		// called "crosvm.sock", find a different way to get the name.
		name := filepath.Base(path)
		// Take the ".sock" off the end of the name. It has to be there, since
		// it's in the Glob pattern.
		name = name[:len(name)-5]
		p.Set(
			perf.Metric{
				Name: fmt.Sprintf("virtio_balloon_%s%s", name, suffix),
				Unit: "MiB",
			},
			float64(stats.BalloonActual)/MiB,
		)
	}
	return nil
}
