// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	allocAnon    = "memory.Alloc.anon_crostini"
	limitReclaim = "memory.Limit.reclaim_crostini"
)

var helperBins = []string{
	allocAnon,
	limitReclaim,
}

var helperArchs = map[string]bool{
	"x86_64": true,
}

func helperFullName(bin, arch string) string {
	return fmt.Sprintf("%s_%s", bin, arch)
}

// HelpersData returns all the data files that are needed to run these Crostini
// memory Alloc and Limit tools.
func HelpersData() []string {
	var data []string
	for _, bin := range helperBins {
		for arch := range helperArchs {
			data = append(data, helperFullName(bin, arch))
		}
	}
	return data
}

// installedHelpers is a map of helper name (e.g. "memory.Alloc.anon_crostini")
// to its location in the Crostini VM
// (e.g. /home/testuser/memory.Alloc.anon_crostini_x86_64)
var installedHelpers map[string]string

// PushHelpers copies all the binaries used to implement Alloc and Limit into
// the Crostini VM.
func PushHelpers(ctx context.Context, cont *vm.Container, dataPath func(string) string) error {
	if installedHelpers != nil {
		return nil
	}

	// Get the architecture of the Crostini container
	archOut, err := cont.Command(ctx, "uname", "-m").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to read Crostini machine hardware name")
	}
	arch := strings.TrimSpace(string(archOut))
	if _, ok := helperArchs[arch]; !ok {
		return errors.Errorf("unsupported architecture %q", arch)
	}

	// Get Crostini's home folder, where we will copy tools.
	user, err := cont.GetUsername(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get Crostini user to compute home folder")
	}
	home := path.Join("/home", user)

	// Push the helpers build with the correct architecture to Crostini.
	installedHelpers = make(map[string]string)
	for _, bin := range helperBins {
		archBin := fmt.Sprintf("%s_%s", bin, arch)
		hostPath := dataPath(archBin)
		crostiniPath := path.Join(home, archBin)
		if err := cont.PushFile(ctx, hostPath, crostiniPath); err != nil {
			return errors.Wrapf(err, "failed to copy %q to Crostini", archBin)
		}
		installedHelpers[bin] = crostiniPath
		if err := cont.Command(ctx, "chmod", "755", crostiniPath).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to chmod %q", crostiniPath)
		}
	}
	return nil
}

// Cleanup kills all running tools and deletes them from Crostini.
func Cleanup(ctx context.Context, cont *vm.Container) error {
	var errRet error
	for _, crostiniPath := range installedHelpers {
		if err := cont.Command(ctx, "killall", "-9", crostiniPath).Run(); err == nil {
			// If there was no error, then it means killall found something to kill.
			testing.ContextLogf(ctx, "Warning: instances of %q still running at Cleanup", crostiniPath)
		}
		if err := cont.Command(ctx, "rm", crostiniPath).Run(testexec.DumpLogOnError); err != nil {
			if errRet == nil {
				errRet = errors.Wrapf(err, "failed to rm %q", crostiniPath)
			} else {
				testing.ContextLogf(ctx, "Failed to rm %q", crostiniPath)
			}
		}
	}
	return errRet
}

// NewAnonAlloc creates a memory.Alloc that allocates anonymous memory in
// Crostini.
// ratio - compressible ratio, [0-1], 0 very compressible, 1 incompressible
// oomScoreAdj - set the allocating process /proc/self/oom_score_adj
func NewAnonAlloc(ctx context.Context, cont *vm.Container, ratio float64, oomScoreAdj int) (*memory.CmdAlloc, error) {
	bin, ok := installedHelpers[allocAnon]
	if !ok {
		return nil, errors.New("crostini allocation helpers are not installed")
	}
	cmd := cont.Command(
		ctx,
		bin,
		fmt.Sprintf("%f", ratio),
		fmt.Sprintf("%d", oomScoreAdj),
	)
	// NB: vm.Container.Command sets Stdin because vsh requires it to be set.
	// NewCmdAlloc will override Stdin, so clear it here so it's safe to set
	// again.
	cmd.Stdin = nil
	return memory.NewCmdAlloc(cmd)
}

// NewPageReclaimLimit creates a memory.Limit that measures if Crostini is
// reclaiming pages and is near to OOMing.
func NewPageReclaimLimit(ctx context.Context, cont *vm.Container) (*memory.CmdLimit, error) {
	bin, ok := installedHelpers[limitReclaim]
	if !ok {
		return nil, errors.New("crostini allocation helpers are not installed")
	}
	cmd := cont.Command(
		ctx,
		bin,
	)
	// NB: vm.Container.Command sets Stdin because vsh requires it to be set.
	// NewCmdLimit will override Stdin, so clear it here so it's safe to set
	// again.
	cmd.Stdin = nil
	return memory.NewCmdLimit(cmd)
}
