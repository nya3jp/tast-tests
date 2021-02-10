// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory"
	arcMemory "chromiumos/tast/local/memory/arc"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type lifecycleParam struct {
	inHost, inARC bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Lifecycle,
		Desc:         "Create many Apps, Tabs, Processes across multiple VMs, and see how many can stay alive",
		Contacts:     []string{"cwd@google.com", "cros-platform-kernel-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "host",
			Pre:  multivm.NoVMStarted(),
			Val:  &lifecycleParam{inHost: true},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "arc",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inARC: true},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "arc_host",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inARC: true, inHost: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "host_with_bg_arc",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inHost: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}},
	})
}

type suspendArcvmTask struct {
}

func arcvmCommand(ctx context.Context, command string) error {
	arcvmSockets, err := filepath.Glob("/run/vm/*/arcvm.sock")
	if err != nil {
		return errors.Wrap(err, "failed to find arcvm sockets")
	}
	if len(arcvmSockets) != 1 {
		return errors.Errorf("expected 1 arcvm socket, got %d", len(arcvmSockets))
	}
	arcvmSocket := arcvmSockets[0]
	if err := testexec.CommandContext(ctx, "crosvm", command, arcvmSocket).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to suspend crosvm at socket %s", arcvmSocket)
	}
	return nil
}

func suspendArcvm(ctx context.Context) error {
	testing.ContextLog(ctx, "Suspending ARCVM")
	if err := arcvmCommand(ctx, "suspend"); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, time.Second*10); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	return nil
}

func resumeArcvm(ctx context.Context) error {
	testing.ContextLog(ctx, "Resuming ARCVM")
	if err := arcvmCommand(ctx, "resume"); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, time.Second*10); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	return nil
}

func (t *suspendArcvmTask) Run(ctx context.Context, _ *memoryuser.TestEnv) error {
	return suspendArcvm(ctx)
}

// Close does nothing.
func (t *suspendArcvmTask) Close(_ context.Context, _ *memoryuser.TestEnv) {
}

// String gives MemoryUser a friendly string for logging.
func (t *suspendArcvmTask) String() string {
	return "Suspend ARCVM"
}

// NeedVM is false because we do not need a new Crostini VM spun up.
func (t *suspendArcvmTask) NeedVM() bool {
	return false
}

func Lifecycle(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*lifecycleParam)
	arc := multivm.ARCFromPre(pre)

	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}

	// Use a PageReclaimLimit to avoid OOMing in the host. Will be composed with
	// VM limits so that we don't OOM in the host or any VM.
	hostLimit := memory.NewPageReclaimLimit()

	var server *memoryuser.MemoryStressServer
	numTypes := 0
	if param.inHost {
		server = memoryuser.NewMemoryStressServer(s.DataFileSystem())
		numTypes++
	}
	if param.inARC {
		numTypes++
	}

	// Created tabs/apps/etc. should have memory that is a bit compressible.
	// We use the same value as the low compress ratio in
	// platform.MemoryStressBasic.
	const compressRatio = 0.67
	const numTasks = 100
	taskAllocMiB := (2 * int64(info.Total) / numTasks) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var tabsAliveTasks []memoryuser.KillableTask
	var appsAliveTasks []memoryuser.KillableTask
	for i := 0; i < numTasks/numTypes; i++ {
		if param.inHost {
			task := server.NewMemoryStressTask(int(taskAllocMiB), compressRatio, hostLimit)
			tabsAliveTasks = append(tabsAliveTasks, task)
			tasks = append(tasks, task)
		}
		if param.inARC {
			task := memoryuser.NewArcLifecycleTask(len(appsAliveTasks), int64(taskAllocMiB)*memory.MiB, compressRatio, hostLimit)
			appsAliveTasks = append(appsAliveTasks, task)
			tasks = append(tasks, task)
		}
		if arc != nil && len(tasks) == 30 {
			tasks = append(tasks, &suspendArcvmTask{})
		}
		if len(tasks) == 0 {
			s.Fatal("No MemoryTasks created")
		}
	}

	if param.inHost {
		task := memoryuser.NewStillAliveMetricTask(tabsAliveTasks, "tabs_alive")
		tasks = append(tasks, task)
	}
	if param.inARC {
		task := memoryuser.NewStillAliveMetricTask(appsAliveTasks, "apps_alive")
		tasks = append(tasks, task)
		if err := memoryuser.InstallArcLifecycleTestApps(ctx, arc, len(appsAliveTasks)); err != nil {
			s.Fatal("Failed to install ArcLifecycleTestApps: ", err)
		}
	}

	p := perf.NewValues()

	// Run all the tasks.
	rp := &memoryuser.RunParameters{
		UseARC:             arc != nil,
		ExistingChrome:     pre.Chrome,
		ExistingARC:        arc,
		ExistingPerfValues: p,
	}
	if err := memoryuser.RunTest(ctx, s.OutDir(), tasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}

	if err := memory.SmapsMetrics(ctx, p, s.OutDir(), ""); err != nil {
		s.Error("Failed to log smaps_rollup metrics: ", err)
	}
	if arc != nil {
		if err := resumeArcvm(ctx); err != nil {
			s.Fatal("Failed to resume arcvm: ", err)
		}
		if err := arcMemory.DumpsysMeminfoMetrics(ctx, arc, p, s.OutDir(), ""); err != nil {
			s.Error("Failed to log dumpsys meminfo metrics: ", err)
		}
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf.Values: ", err)
	}
}
