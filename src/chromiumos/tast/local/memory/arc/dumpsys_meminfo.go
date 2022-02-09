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
	"chromiumos/tast/testing"
)

// SliceToPssMap maps a slice of processes/categories into a Pss memory metric.
type SliceToPssMap map[string]uint64

// VMSummary holds overall information on metrics from a VM.
// All values in Kilobytes.
type VMSummary struct {
	UsedPss          uint64
	KernelPss        uint64
	CachedKernel     uint64
	CachedPss        uint64
	CategoryPss      SliceToPssMap
	ProcessPss       SliceToPssMap
	DetailedPssUsage uint64
	FreeRAM          int64
	LostRAM          int64
}

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
// Sample line: Free RAM: 1,002,789K (  195,493K cached pss +   449,008K cached kernel +   358,288K free)
var freeRAMRE = regexp.MustCompile(
	`(?m)` + // Allow parsing multiple lines;
		`^[ \t]*Free RAM:[ \t]*[0-9][0-9,]*K` + // Free Ram, up to K
		`[ \t]*\([ \t]*` + // Match space, open parenthesis, space;
		`[0-9][0-9,]*K[ \t]*cached pss` + // match "9,999K cached pss"
		`[ \t]*\+[ \t]*` + // Match space, plus sign, space;
		`[0-9][0-9,]*K[ \t]*cached kernel`)

// lostRAMRE is a regex to match the Lost RAM line of dumpinfo's summary.
var lostRAMRE = regexp.MustCompile(`(?m)^[ \t]*Lost RAM:[ \t]*\-?[0-9][0-9,]*K`)

// memDetailPercentage defines how much % of PSS consumption will be
// listed in details in metrics.
const (
	memDetailPercentage = 80
	KiBInMiB            = 1024
)

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

func parseNumBetweenMarkers(s, leftMarker, rightMarker string) (int64, error) {
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
	return strconv.ParseInt(numkstr, 10, 64)
}

// GetDumpsysMeminfoMetrics parses several key metrics from the output
// of  `dumpsys meminfo` into the returned VMSummary struct,
// including PSS metrics grouped per app and per category.
// The raw `dumpsys meminfo` text is additionally written to a file
// for debug purposes - provided that outdir is not empty.
func GetDumpsysMeminfoMetrics(ctx context.Context, a *arc.ARC, outdir, suffix string) (*VMSummary, error) {
	meminfo, err := a.Command(ctx, "dumpsys", "meminfo").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run \"dumpsys meminfo\"")
	}

	errorContext := func() string {
		testing.ContextLogf(ctx, "Failed to parse 'dumpsys meminfo' output: %s", string(meminfo))
		return "see log for 'dumpsys meminfo' output"
	}

	// Keep a copy in logs for debugging.
	if len(outdir) > 0 {
		outfile := "arc.meminfo" + suffix + ".txt"
		outpath := filepath.Join(outdir, outfile)
		if err := ioutil.WriteFile(outpath, meminfo, 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write meminfo to %q", outpath)
		}
		errorContext = func() string {
			return fmt.Sprintf("see %q for 'dumpsys meminfo' output", outpath)
		}
	}

	// Extract the position of the "Total PSS by process" section.
	pos := pssByProcessPosRE.FindIndex(meminfo)
	if pos == nil {
		return nil, errors.Errorf("failed to find 'Total PSS by process' section, %s", errorContext())
	}
	matches := processPssRE.FindAllSubmatch(meminfo[pos[0]:pos[1]], -1)

	if matches == nil {
		return nil, errors.Errorf("unable to parse meminfo, %s", errorContext())
	}

	metrics := make(SliceToPssMap)
	for _, match := range matches {
		pss, err := strconv.ParseUint(strings.ReplaceAll(string(match[1]), ",", ""), 10, 64)
		if err != nil {
			return nil, errors.Errorf("unable to parse meminfo line %q, %s", match[0], errorContext())
		}
		for _, category := range appCategories {
			if category.appRE.Match(match[2]) {
				metrics[category.name] += pss
				// Only the first matching category should contain this process.
				break
			}
		}
	}

	pos = freeRAMRE.FindIndex(meminfo)
	if pos == nil {
		return nil, errors.Errorf("failed to find 'Free RAM' section, %s", errorContext())
	}
	freeRAM, err := parseNumBetweenMarkers(string(meminfo[pos[0]:pos[1]]), ":", "K")
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse Free RAM section, %s", errorContext())
	}

	cachedPss, err := parseNumBetweenMarkers(string(meminfo[pos[0]:pos[1]]), "(", "K")
	if err != nil {
		return nil, errors.Wrapf(err, "could not get cachedPSS from Free RAM section, %s", errorContext())
	}

	cachedKernel, err := parseNumBetweenMarkers(string(meminfo[pos[0]:pos[1]]), "+", "K")
	if err != nil {
		return nil, errors.Wrapf(err, "could not get cachedKernel from Free RAM section, %s", errorContext())
	}

	pos = lostRAMRE.FindIndex(meminfo)
	if pos == nil {
		return nil, errors.Errorf("failed to find 'Lost RAM' section, %s", errorContext())
	}
	lostRAM, err := parseNumBetweenMarkers(string(meminfo[pos[0]:pos[1]]), ":", "K")
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse Lost RAM section, %s", errorContext())
	}

	pos = usedMemoryTotalsRE.FindIndex(meminfo)
	if pos == nil {
		return nil, errors.Errorf("failed to find 'Used RAM' section, %s", errorContext())
	}

	usedRAMText := string(meminfo[pos[0]:pos[1]])

	usedPssTotal, err := parseNumBetweenMarkers(usedRAMText, "(", "K")
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find PSS total, %s", errorContext())
	}

	kernelTotal, err := parseNumBetweenMarkers(usedRAMText, "+", "K")
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find Kernel total, %s", errorContext())
	}

	pos = pssByCategoryPosRE.FindIndex(meminfo)
	if pos == nil {
		return nil, errors.Errorf("failed to find 'Total PSS by category' section, %s", errorContext())
	}

	vmSummary := &VMSummary{
		UsedPss:      uint64(usedPssTotal),
		KernelPss:    uint64(kernelTotal),
		CachedPss:    uint64(cachedPss),
		CachedKernel: uint64(cachedKernel),
		CategoryPss:  make(SliceToPssMap),
		LostRAM:      lostRAM,
		FreeRAM:      freeRAM,
		ProcessPss:   metrics,
	}

	// Parse all categories of memory consumption.
	// Categories are listed in descending order in the input,
	// and the total of all of them is in "used pss", which we
	// parsed earlier into usedPssTotal.
	catglines := strings.Split(string(meminfo[pos[0]:pos[1]]), "\n")
	var detailedPssUsage uint64
	var detailThreshold = (vmSummary.UsedPss * memDetailPercentage) / 100
	for _, line := range catglines[1:] {
		kix := strings.Index(line, "K: ")
		if kix < 0 {
			return nil, errors.Errorf("unable to parse category line %q, %s", line, errorContext())
		}
		numkstr := strings.ReplaceAll(strings.TrimSpace(line[:kix]), ",", "")
		numk, err := strconv.ParseUint(numkstr, 10, 64)
		if err != nil {
			return nil, errors.Errorf("failed to parse category memory size %q, %s", numkstr, errorContext())
		}
		name := strings.ReplaceAll(line[kix+3:], ".", "")
		name = strings.ReplaceAll(name, " ", "_")

		if detailedPssUsage < detailThreshold {
			detailedPssUsage += numk
			vmSummary.CategoryPss[name] = numk
		}
	}
	// The part that is not categorized
	if vmSummary.UsedPss > detailedPssUsage { // Should always be so.
		vmSummary.CategoryPss["others"] = (vmSummary.UsedPss - detailedPssUsage)
	}

	return vmSummary, nil
}

// ReportDumpsysMeminfoMetrics outputs a set of representative metrics
// into the supplied performance data dictionary.
func ReportDumpsysMeminfoMetrics(vmSummary *VMSummary, p *perf.Values, suffix string) {

	// All categories
	for name, numk := range vmSummary.CategoryPss {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("arc_category_%s_pss%s", name, suffix),
				Unit:      "KiB",
				Direction: perf.SmallerIsBetter,
			},
			float64(numk),
		)
	}

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_used_pss%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(vmSummary.UsedPss),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_kernel_ram%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(vmSummary.KernelPss),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_free_ram%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(vmSummary.FreeRAM),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_lost_ram%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(vmSummary.LostRAM),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("arc_total_used_pss%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(vmSummary.UsedPss+vmSummary.KernelPss),
	)

	for name, value := range vmSummary.ProcessPss {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s_pss%s", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			// Q: Wy not use KiB for this one too?
			// A: This is historically reported in MiB, keeping it that way
			//    to avoid losing continuity.
			float64(value)/KiBInMiB,
		)
	}
}
