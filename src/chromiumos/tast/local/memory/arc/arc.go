// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/testexec"
)

const (
	allocAnon    = "memory.Alloc.anon_arc"
	limitReclaim = "memory.Limit.reclaim_arc"
)

var helperBins = []string{
	allocAnon,
	limitReclaim,
}

var helperArchs = map[string]bool{
	"x86_64": true,
	"arm64":  true,
}

func helperFullName(bin, arch string) string {
	return fmt.Sprintf("%s_%s", bin, arch)
}

// HelpersData returns the list of data files needed by tests to run these
// helpers.
func HelpersData() []string {
	var data []string
	for _, bin := range helperBins {
		for arch := range helperArchs {
			data = append(data, helperFullName(bin, arch))
		}
	}
	return data
}

func pushHelper(ctx context.Context, a *arc.ARC, hostPath string) (string, error) {
	arcPath, err := a.PushFileToTmpDir(ctx, hostPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to push %q to tmpdir", hostPath)
	}
	if err := a.Command(ctx, "chmod", "0755", arcPath).Run(testexec.DumpLogOnError); err != nil {
		return arcPath, errors.Wrapf(err, "failed to change the permission of %q", arcPath)
	}
	return arcPath, nil
}

var installedHelpers map[string]string

// PushHelpers copies the helper data files of the correct architecture to ARC.
func PushHelpers(ctx context.Context, a *arc.ARC, dataPath func(string) string) error {
	if installedHelpers != nil {
		return nil
	}
	installedHelpers = make(map[string]string)

	archOut, err := a.Command(ctx, "getprop", "ro.product.cpu.abi").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to read android cpu abi")
	}
	arch := strings.TrimSpace(string(archOut))

	for _, bin := range helperBins {
		arcBin, err := pushHelper(ctx, a, dataPath(helperFullName(bin, arch)))
		if arcBin != "" {
			installedHelpers[bin] = arcBin
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// NewAnonAlloc creates a memory.Alloc that allocates anonymous memory in ARC.
// ratio - compressible ratio, [0-1], 0 very compressible, 1 incompressible
// oomScoreAdj - set the allocating process /proc/self/oom_score_adj
func NewAnonAlloc(ctx context.Context, a *arc.ARC, ratio float64, oomScoreAdj int) (*memory.CmdAlloc, error) {
	bin, ok := installedHelpers[allocAnon]
	if !ok {
		return nil, errors.New("arc allocation helpers are not installed")
	}
	return memory.NewCmdAlloc(a.Command(
		ctx,
		bin,
		fmt.Sprintf("%f", ratio),
		fmt.Sprintf("%d", oomScoreAdj),
	))
}

// NewPageReclaimLimit creates a memory.Limit that measures if ARC is reclaiming
// pages and is near to OOMing.
func NewPageReclaimLimit(ctx context.Context, a *arc.ARC) (*memory.CmdLimit, error) {
	bin, ok := installedHelpers[limitReclaim]
	if !ok {
		return nil, errors.New("arc allocation helpers are not installed")
	}
	return memory.NewCmdLimit(a.Command(
		ctx,
		bin,
	))
}
