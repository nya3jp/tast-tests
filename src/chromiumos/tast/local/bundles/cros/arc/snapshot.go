// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/snapshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Snapshot,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that the date command prints dates as expected",
		Contacts:     []string{"arc-core@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"android_vm", "chrome"},
	})
}

func Snapshot(ctx context.Context, s *testing.State) {
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to open syslog reader: ", err)
	}
	defer reader.Close()

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while booting ARC: ", err)
		}
	}()

	a, err := arc.NewWithSyslogReader(ctx, s.OutDir(), reader)
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	basemem, err := metrics.NewBaseMemoryStats(ctx, a)
	if err != nil {
		s.Fatal("Failed to retrieve base memory stats: ", err)
	}
	p := perf.NewValues()

	snapshotPath, err := snapshot.GetPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get snapshot path: ", err)
	}
	s.Log("Snapshot path: ", snapshotPath)

	socketPath, err := getCrosvmSocketPath()
	if err != nil {
		s.Fatal("Failed to get crosvm sock: ", err)
	}
	s.Log("Socket: ", socketPath)

	if status, err := snapshot.GetStatus(ctx, socketPath); err != nil {
		s.Fatal("Failed to get snapshot status: ", err)
	} else if status != snapshot.NotAvailable {
		s.Fatal("snapshot status is not NotAvailable but: ", status)
	}

	if err := snapshot.Take(ctx, socketPath, snapshotPath); err != nil {
		s.Fatal("Failed to take snapshot: ", err)
	}

	s.Log("Waiting snapshot completes")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := snapshot.GetStatus(ctx, socketPath); err != nil {
			return err
		} else if status == snapshot.Done {
			return nil
		} else if status == snapshot.Failed {
			return testing.PollBreak(errSnapshot)
		}
		return errSnapshotInProgress
	}, nil)
	if err != nil {
		s.Fatal("Snapshot not complete: ", err)
	}

	s.Log("Snapshot taken")
	// Do not collect metrics from ARC because while the VM is suspended from snapshot, adb does not respond and collecting metrics from ARC fails.
	if err := metrics.LogMemoryStats(ctx, basemem, nil, p, s.OutDir(), "_snapshot"); err != nil {
		s.Error("Failed to collect memory metrics: ", err)
	}

	s.Log("Resume crosvm")
	if err := snapshot.Resume(ctx, socketPath); err != nil {
		s.Fatal("Failed to resume crosvm: ", err)
	}

	err = testing.Poll(ctx, func(ctx context.Context) error {
		return a.IsConnected(ctx)
	}, nil)
	if err != nil {
		s.Fatal("ARC is offline: ", err)
	}
	s.Log("Crosvm is running")
	if err := metrics.LogMemoryStats(ctx, basemem, nil, p, s.OutDir(), "_resume"); err != nil {
		s.Error("Failed to collect memory metrics: ", err)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf.Values: ", err)
	}
}

var (
	errCrosvmNotFound     = errors.New("crosvm process not found")
	errSnapshot           = errors.New("crosvm failed to take snapshot")
	errSnapshotInProgress = errors.New("crosvm snapshot in progress")
)

func getCrosvmSocketPath() (string, error) {
	procs, err := process.Processes()
	if err != nil {
		return "", errors.Wrap(err, "failed to get process list")
	}
	for _, proc := range procs {
		cmdline, err := proc.CmdlineSlice()
		if err != nil {
			return "", errors.Wrap(err, "failed to get cmdline for a process")
		} else if len(cmdline) == 0 {
			continue
		} else if cmdline[0] != "/usr/bin/crosvm" {
			continue
		}
		for i, arg := range cmdline {
			if arg == "--socket" && i+1 < len(cmdline) {
				return cmdline[i+1], nil
			}
		}
	}
	return "", errCrosvmNotFound
}
