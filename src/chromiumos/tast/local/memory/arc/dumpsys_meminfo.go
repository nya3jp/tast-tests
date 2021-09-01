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

// pssByCategoryPosRE matches all lines in `dumpsys meminfo` that contain per-category PSS information.
var pssByCategoryPosRE = regexp.MustCompile(`(?m)^Total PSS by category:(\n *[0-9][0-9,]*K: [^\n]+)+`)

// usedMemoryTotalsRE is a regex to match dumpinfo's summary line
// Sample line: " Used RAM:   528,689K (  422,021K used pss +   106,668K kernel)".
var usedMemoryTotalsRE = regexp.MustCompile(
	`(?m)` + // Allow parsing multiple lines;
		`^[ \t]*Used RAM:[ \t]*[0-9][0-9,]*K` + // Match "Used RAM:   9,999K";
		`[ \t]*\([ \t]*` + // Match space, open parenthesis, space;
		`[0-9][0-9,]*K[ \t]*used pss` + // Match "9,999K used pss +";
		`[ \t]*\+[ \t]*` + // Match space, plus sign, space;
		`[0-9][0-9,]*K[ \t]*kernel`) // Match "9,999K kernel".

// freeRAMRE is a regex to match the Free RAM line of dumpinfo's summary.
var freeRAMRE = regexp.MustCompile(`(?m)^[ \t]*Free RAM:[ \t]*[0-9][0-9,]*K`)

// lostRAMRE is a regex to match the Lost RAM line of dumpinfo's summary.
var lostRAMRE = regexp.MustCompile(`(?m)^[ \t]*Lost RAM:[ \t]*[0-9][0-9,]*K`)

// memDetailPercentage defines how much % of PSS consumption will be
// listed in details in metrics.
const memDetailPercentage = 80

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

func parseNumBetweenMarkers(s, leftMarker, rightMarker string) (uint64, error) {
	preix := strings.Index(s, leftMarker)
	if preix < 0 {
		return 0, errors.New("cannot enclosing left marker")
	}
	pastpreix := preix + len(leftMarker)
	postix := strings.Index(s[pastpreix:], rightMarker)
	if postix < 0 {
		return 0, errors.New("cannot enclosing right marker")
	}
	numkstr := s[pastpreix : pastpreix+postix]
	numkstr = strings.ReplaceAll(strings.TrimSpace(numkstr), ",", "")
	return strconv.ParseUint(numkstr, 10, 64)
}

// DumpsysMeminfoMetrics write the output of `dumpsys meminfo` to outdir. If p
// is provided, it adds PSS metrics for each of the app categories defined in
// appCategories above. If outdir is "", then no logs are written.
func DumpsysMeminfoMetrics(ctx context.Context, a *arc.ARC, p *perf.Values, outdir, suffix string) error {
	meminfo, err := a.Command(ctx, "dumpsys", "meminfo").Output()
	if err != nil {
		return errors.Wrap(err, "failed to run \"dumpsys meminfo\"")
	}

	// Keep a copy in logs for debugging.
	if len(outdir) > 0 {
		outfile := "arc.meminfo" + suffix + ".txt"
		outpath := filepath.Join(outdir, outfile)
		if err := ioutil.WriteFile(outpath, meminfo, 0644); err != nil {
			return errors.Wrapf(err, "failed to write meminfo to %q", outpath)
		}
	}

	if p == nil {
		// No perf.Values, so don't compute metrics.
		return nil
	}

	// Extract the position of the "Total PSS by process" section.
	pos := pssByProcessPosRE.FindIndex(meminfo)
	if pos == nil {
		return errors.New("failed to find 'Total PSS by process' section")
	}
	matches := processPssRE.FindAllSubmatch(meminfo[pos[0]:pos[1]], -1)

	if matches == nil {
		return errors.New("unable to parse meminfo")
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

	pos = freeRAMRE.FindIndex(meminfo)
	if pos == nil {
		return errors.New("failed to find 'Free RAM' section")
	}
	freeRAM, err := parseNumBetweenMarkers(string(meminfo[pos[0]:pos[1]]), ":", "K")
	if err != nil {
		return errors.Wrap(err, "unable to parse Free RAM section")
	}

	pos = lostRAMRE.FindIndex(meminfo)
	if pos == nil {
		return errors.New("failed to find 'Lost RAM' section")
	}
	lostRAM, err := parseNumBetweenMarkers(string(meminfo[pos[0]:pos[1]]), ":", "K")
	if err != nil {
		return errors.Wrap(err, "unable to parse Lost RAM section")
	}

	pos = usedMemoryTotalsRE.FindIndex(meminfo)
	if pos == nil {
		return errors.New("Unable to match regex")
	}

	usedRAMText := string(meminfo[pos[0]:pos[1]])

	usedPssTotal, err := parseNumBetweenMarkers(usedRAMText, "(", "K")
	if err != nil {
		return errors.Wrap(err, "unable to find PSS total")
	}

	kernelTotal, err := parseNumBetweenMarkers(usedRAMText, "+", "K")
	if err != nil {
		return errors.Wrap(err, "unable to find Kernel total")
	}

	pos = pssByCategoryPosRE.FindIndex(meminfo)
	if pos == nil {
		return errors.New("failed to find 'Total PSS by category' section")
	}

	// Parse all categories of memory consumption.
	// Categories are listed in descending order in the input,
	// and the total of all of them is in "used pss", which we
	// parsed earlier into usedPssTotal.
	catglines := strings.Split(string(meminfo[pos[0]:pos[1]]), "\n")
	var detailedPssUsage uint64
	var detailThreshold = (usedPssTotal * memDetailPercentage) / 100
	for _, line := range catglines[1:] {
		kix := strings.Index(line, "K: ")
		if kix < 0 {
			return errors.New("unable to parse category line [" + line + "]")
		}
		numkstr := strings.ReplaceAll(strings.TrimSpace(line[:kix]), ",", "")
		numk, err := strconv.ParseUint(numkstr, 10, 64)
		if err != nil {
			return errors.New("failed to parse category memory size [" + numkstr + "]")
		}
		name := strings.ReplaceAll(line[kix+3:], ".", "")
		name = strings.ReplaceAll(name, " ", "_")

		if detailedPssUsage < detailThreshold {
			detailedPssUsage += numk
			p.Set(
				perf.Metric{
					Name:      fmt.Sprintf("arc_category_%s_pss%s", name, suffix),
					Unit:      "MiB",
					Direction: perf.SmallerIsBetter,
				},
				float64(numk),
			)
		}
	}

	if usedPssTotal > detailedPssUsage { // Should always be so.
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("arc_category_others_pss%s", suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			float64(usedPssTotal-detailedPssUsage),
		)
	}

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_used_pss%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(usedPssTotal),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_kernel_ram%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(kernelTotal),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_free_ram%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(freeRAM),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_lost_ram%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(lostRAM),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_total_used_pss%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(usedPssTotal+kernelTotal),
	)

	for name, value := range metrics {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s_pss%s", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			value,
		)
	}
	return nil
}
