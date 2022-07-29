package multivm

import (
	//"strconv"
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type canaryHealthPerfParam struct {
	allocationTarget memoryuser.AllocationTarget
	canary           memoryuser.CanaryType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryAllocationCanaryHealthPerf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "How much memory can we allocate before the specified canary dies",
		Contacts: []string{
			"kokiryu@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "tab_host",
			Pre:  multivm.NoVMStarted(),
			Val:  &canaryHealthPerfParam{memoryuser.Host, memoryuser.Tab},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "tab_arc",
			Pre:               multivm.ArcStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.Arc, memoryuser.Tab},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_host",
			Pre:               multivm.ArcStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.Arc, memoryuser.App},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_arc",
			Pre:               multivm.ArcStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.Arc, memoryuser.App},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Vars: []string{
			"iterations",
		},
		Timeout: 30 * time.Minute,
	})
}
func MemoryAllocationCanaryHealthPerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*canaryHealthPerfParam)
	preARC := multivm.ARCFromPre(pre)

	//iterationsStr, ok := s.Var("iterations")
	//var iterations int
	//if ok {
	//	iterationsConv, err := strconv.Atoi(iterationsStr)
	//	if err != nil {
	//		s.Fatal("Could not convert the iterations arg to integer: ", err)
	//	}
	//	iterations = iterationsConv
	//} else {
	//	iterations = 5
	//}

	basemem, err := metrics.NewBaseMemoryStats(ctx, preARC)
	if err != nil {
		s.Fatal("Failed to retrieve base memory stats: ", err)
	}

	var allocatedMiB int64
	allocatedMiB = 0

	iterations := 1
	for i := 0; i < iterations; i++ {
		canary, err := memoryuser.NewCanary(param.canary, ctx, 50, 0.67, s, pre.Chrome, preARC)
		if err != nil {
			s.Fatal("Failed to create the canary: ", err)
		}
		canary.Run(ctx)

		target := param.allocationTarget
		allocationManager := memoryuser.NewMemoryAllocationManager(target, 250, 0.67, preARC)
		allocationManager.Setup(ctx)

		for true {
			if !canary.StillAlive(ctx) {
				s.Logf("%s died after %d MiB allocations", canary.String(), allocationManager.TotalAllocatedMiB())
				allocatedMiB += allocationManager.TotalAllocatedMiB()
				break
			}
			if id := allocationManager.DeadAllocator(); id >= 0 {
				s.Fatalf("Allocator %d is killed before the canary", id)
			}
			if err := allocationManager.AddAllocator(ctx); err != nil {
				s.Fatal("Failed to add an allocator: ", err)

			}
		}
		canary.Close(ctx)
		allocationManager.Cleanup(ctx)
	}

	p := perf.NewValues()
	if err := metrics.LogMemoryStats(ctx, basemem, preARC, p, s.OutDir(), ""); err != nil {
		s.Error("Failed to collect memory metrics: ", err)
	}
	p.Set(
		perf.Metric{
			Name:      "allocatedMiB",
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(allocatedMiB)/float64(iterations),
	)
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf.Values: ", err)
	}
}
