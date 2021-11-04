// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory/kernelmeter"
)

// VMStatMetrics writes the contents of `/proc/vmstat` to outdir. If outdir is
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

	m, err := kernelmeter.ParseVMStats(string(vmstat))
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
