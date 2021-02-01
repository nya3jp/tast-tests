// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
)

// DumpsysMeminfo writes the results of `adb shell dumpsys meminfo` to
// arc.meminfo.log in outdir.
func DumpsysMeminfo(ctx context.Context, a *arc.ARC, outdir string) error {
	meminfo, err := a.Command(ctx, "dumpsys", "meminfo").Output()
	if err != nil {
		return errors.Wrap(err, "failed to run \"dumpsys meminfo\"")
	}

	const outfile = "arc.meminfo.log"
	outpath := filepath.Join(outdir, outfile)
	if err := ioutil.WriteFile(outpath, meminfo, 0644); err != nil {
		return errors.Wrapf(err, "failed to write meminfo to %q", outpath)
	}
	return nil
}

// pssByProcessPosRE matches all lines in `dumpsys meminfo` output that contain
// per process information on PSS. Used with FindIndex to extract the range of
// lines to make it easier to get per-process PSS information.
var pssByProcessPosRE = regexp.MustCompile(`(?m)^Total PSS by process:(\n *[0-9][0-9,]*K: [^\n]+)+`)

// processPssRE matches a single line under a `Total (PSS|RSS) by .*:` heading.
// Match groups:
// 1 - the memory size in KB
// 2 - the name of the process
var processPssRE = regexp.MustCompile(` *([0-9][0-9,]*)K: (.*) \(pid ([0-9]+(?: / activities)?)\)`)

type appCategory struct {
	appRE *regexp.Regexp
	name  string
}

// appCategories defines categories used to aggregate per-process memory
// metrics. The first appRE to match an app defines its category.
var appCategories = []appCategory{
	{
		appRE: regexp.MustCompile(`^org\.chromium\.arc\.testapp\.lifecycle`),
		name:  "arc_lifecycle",
	}, {
		appRE: regexp.MustCompile(`^com\.android\.`),
		name:  "arc_com.android",
	}, {
		appRE: regexp.MustCompile(`^com\.google\.`),
		name:  "arc_com.google",
	}, {
		appRE: regexp.MustCompile(`^org\.chromium\.`),
		name:  "arc_org.chromium",
	}, {
		appRE: regexp.MustCompile(`.*`),
		name:  "arc_other",
	},
}

// DumpsysMeminfoMetrics write the output of `dumpsys meminfo` to outdir. If p
// is provided, it adds PSS metrics for each of the app categories defined in
// appCategories above.
func DumpsysMeminfoMetrics(ctx context.Context, a *arc.ARC, p *perf.Values, outdir, suffix string) error {
	meminfo, err := a.Command(ctx, "dumpsys", "meminfo").Output()
	if err != nil {
		return errors.Wrap(err, "failed to run \"dumpsys meminfo\"")
	}

	// Keep a copy in logs for debugging.
	outfile := "arc.meminfo" + suffix + ".txt"
	outpath := filepath.Join(outdir, outfile)
	if err := ioutil.WriteFile(outpath, meminfo, 0644); err != nil {
		return errors.Wrapf(err, "failed to write meminfo to %q", outpath)
	}

	if p == nil {
		// No perf.Values, so don't compute metrics.
		return nil
	}

	// Extract the position of the "Total PSS by process" section.
	pos := pssByProcessPosRE.FindIndex(meminfo)
	if pos == nil {
		return errors.Errorf("failed to find 'Total PSS by process' section in %s in outdir", outfile)
	}
	matches := processPssRE.FindAllSubmatch(meminfo[pos[0]:pos[1]], -1)

	if matches == nil {
		return errors.Errorf("unable to parse meminfo, see %s in outdir", outfile)
	}

	metrics := make(map[string]float64)
	for _, match := range matches {
		pss, err := strconv.ParseUint(strings.ReplaceAll(string(match[1]), ",", ""), 10, 64)
		if err != nil {
			return errors.Errorf("unable to parse meminfo line %q", match[0])
		}
		for _, category := range appCategories {
			if category.appRE.Match(match[2]) {
				metrics[category.name] += float64(pss) / 1024
				// Only the first matching category should contain this process.
				break
			}
		}
	}

	for name, value := range metrics {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_pss", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			value,
		)
	}
	return nil
}
