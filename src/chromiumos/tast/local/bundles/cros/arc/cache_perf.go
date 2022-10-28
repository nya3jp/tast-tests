// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type cacheMode int

const (
	// Use all caches.
	cacheNormal cacheMode = iota
	// Skip using app caches like packages and GMS Core.
	cacheSkipApps
	// Skip using disk caches like ureadahead.
	cacheSkipDisk
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CachePerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measure benefits of using pre-generated caches for package manager and GMS Core",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      30 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		VarDeps: []string{"arc.perfAccountPool"},
	})
}

// CachePerf steps through opt-in flow 5 times, caches enabled and caches disabled.
// Difference for Play Store shown times in case package manager, GMS Core and other caches
// enabled/disabled are tracked. This difference shows the benefit using the pre-generated
// caches. Results are:
//   - Average time difference for ARC optin in seconds
//   - Average time difference for ARC optin in percents relative to the boot with caches on.
//
// Optional (in case RAPL interface is available)
//   - Average energy difference required for ARC optin in joules.
//   - Average energy difference required for ARC optin in percents relative to the boot with caches on.
func CachePerf(ctx context.Context, s *testing.State) {
	const (
		// successBootCount is the number of passing ARC boots to collect results.
		successBootCount = 3

		// maxErrorBootCount is the number of maximum allowed boot errors.
		// used for reliability against optin flow flakiness.
		maxErrorBootCount = 1
	)

	normalTime := time.Duration(0)
	skipAppsTime := time.Duration(0)
	skipDiskTime := time.Duration(0)
	normalEnergy := float64(0)
	skipAppsEnergy := float64(0)
	skipDiskEnergy := float64(0)
	passedCount := 0
	errorCount := 0

	for passedCount < successBootCount {
		normalTimeIter, skipAppsTimeIter, skipDiskTimeIter, normalEnergyIter, skipAppsEnergyIter, skipDiskEnergyIter, err := performIteration(ctx, s)
		if err != nil {
			s.Log("Error found during the ARC boot: ", err)
			errorCount++
			if errorCount > maxErrorBootCount {
				s.Fatalf("Too many(%d) ARC boot errors", errorCount)
			}
			continue
		}
		passedCount++
		normalTime += normalTimeIter
		skipAppsTime += skipAppsTimeIter
		skipDiskTime += skipDiskTimeIter
		normalEnergy += normalEnergyIter
		skipAppsEnergy += skipAppsEnergyIter
		skipDiskEnergy += skipDiskEnergyIter
	}

	normalTime /= time.Duration(passedCount)
	skipAppsTime /= time.Duration(passedCount)
	skipDiskTime /= time.Duration(passedCount)
	normalEnergy /= float64(passedCount)
	skipAppsEnergy /= float64(passedCount)
	skipDiskEnergy /= float64(passedCount)
	boostApps := skipAppsTime - normalTime
	boostDisk := skipDiskTime - normalTime
	percentsApps := 100.0 * boostApps.Seconds() / normalTime.Seconds()
	percentsDisk := 100.0 * boostDisk.Seconds() / normalTime.Seconds()
	s.Logf("Time performance: %.1fs (%.1f%%) for apps caches and %.1fs (%.1f%%) for disk caches  based on %d passed and %d skipped iterations",
		boostApps.Seconds(), percentsApps,
		boostDisk.Seconds(), percentsDisk,
		passedCount, errorCount)

	perfValues := perf.NewValues()
	perfValues.Set(perf.Metric{
		Name:      "boostTime",
		Unit:      "seconds",
		Direction: perf.BiggerIsBetter,
	}, boostApps.Seconds())
	perfValues.Set(perf.Metric{
		Name:      "boostTimePercents",
		Unit:      "percents",
		Direction: perf.BiggerIsBetter,
	}, percentsApps)
	perfValues.Set(perf.Metric{
		Name:      "boostTimeDisk",
		Unit:      "seconds",
		Direction: perf.BiggerIsBetter,
	}, boostDisk.Seconds())
	perfValues.Set(perf.Metric{
		Name:      "boostTimeDiskPercents",
		Unit:      "percents",
		Direction: perf.BiggerIsBetter,
	}, percentsDisk)

	if normalEnergy != 0 {
		boostAppsEnergy := skipAppsEnergy - normalEnergy
		percentsApps = 100.0 * boostAppsEnergy / normalEnergy
		boostDiskEnergy := skipDiskEnergy - normalEnergy
		percentsDisk = 100.0 * boostDiskEnergy / normalEnergy
		s.Logf("Energy performance: %.1f joules (%.1f%%) for apps caches and %.1f joules (%.1f%%) for disk caches  based on %d passed and %d skipped iterations",
			boostAppsEnergy, percentsApps,
			boostDiskEnergy, percentsDisk,
			passedCount, errorCount)
		perfValues.Set(perf.Metric{
			Name:      "boostEnergy",
			Unit:      "joules",
			Direction: perf.BiggerIsBetter,
		}, boostAppsEnergy)
		perfValues.Set(perf.Metric{
			Name:      "boostEnergyPercents",
			Unit:      "percents",
			Direction: perf.BiggerIsBetter,
		}, percentsApps)
		perfValues.Set(perf.Metric{
			Name:      "boostEnergyDisk",
			Unit:      "joules",
			Direction: perf.BiggerIsBetter,
		}, boostDiskEnergy)
		perfValues.Set(perf.Metric{
			Name:      "boostEnergyDiskPercents",
			Unit:      "percents",
			Direction: perf.BiggerIsBetter,
		}, percentsDisk)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// performIteration performs ARC provisioning in two modes, with and without caches.
// It returns durations spent for each boot.
func performIteration(ctx context.Context, s *testing.State) (normalTime, skipAppsTime, skipDiskTime time.Duration, normalEnergy, skipAppsEnergy, skipDiskEnergy float64, err error) {
	skipAppsTime, skipAppsEnergy, err = bootARCCachePerf(ctx, s, cacheSkipApps)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	skipDiskTime, skipDiskEnergy, err = bootARCCachePerf(ctx, s, cacheSkipDisk)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	normalTime, normalEnergy, err = bootARCCachePerf(ctx, s, cacheNormal)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	s.Logf("Iteration done, normal time/energy: %.1fs/%.1fj, boost apps %.1fs/%.1fj, boost disks %.1fs/%.1fj",
		normalTime.Seconds(), normalEnergy,
		(skipAppsTime - normalTime).Seconds(),
		skipAppsEnergy-normalEnergy,
		(skipDiskTime - normalTime).Seconds(),
		skipDiskEnergy-normalEnergy)
	return normalTime, skipAppsTime, skipDiskTime, normalEnergy, skipAppsEnergy, skipDiskEnergy, nil
}

// bootARCCachePerf performs Chrome login and boots ARC. It waits for Play Store is shown and
// reports time elapsed from enabling ARC and Play Store is finally shown.
func bootARCCachePerf(ctx context.Context, s *testing.State, mode cacheMode) (time.Duration, float64, error) {
	// TODO(crbug.com/995869): Remove set of flags to disable app sync, PAI, locale sync, Play Store auto-update.
	args := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui", "--ignore-arcvm-dev-conf")

	switch mode {
	case cacheNormal:
	case cacheSkipDisk:
		// Disabling ureadahead caches.
		args = append(args, "--arc-disable-ureadahead")
	case cacheSkipApps:
		// Disabling package manager cache disables GMS Core cache as well.
		args = append(args, "--arc-packages-cache-mode=skip-copy")
	default:
		return 0, 0, errors.New("invalid cache mode")
	}

	// Drop file caches if any.
	if err := disk.DropCaches(ctx); err != nil {
		return 0, 0, errors.Wrap(err, "failed to drop caches")
	}

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.ARCSupported(),
		chrome.GAIALoginPool(s.RequiredVar("arc.perfAccountPool")),
		chrome.ExtraArgs(args...))
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to login to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, 0, errors.Wrap(err, "creating test API connection failed")
	}

	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		return 0, 0, errors.Wrap(err, "failed to wait CPU cool down")
	}

	energyBefore, err := power.NewRAPLSnapshot()
	if err != nil {
		s.Log("Energy status is not available for this board")
	}
	startTime := time.Now()

	s.Log("Waiting for ARC opt-in flow to complete")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return 0, 0, errors.Wrap(err, "failed to perform optin")
	}

	s.Log("Waiting for Play Store window to be shown")
	if err := optin.WaitForPlayStoreShown(ctx, tconn, 2*time.Minute); err != nil {
		return 0, 0, errors.Wrap(err, "failed to wait Play Store shown")
	}

	duration := time.Now().Sub(startTime)
	energy := float64(0)
	if energyBefore != nil {
		energyDif, err := energyBefore.DiffWithCurrentRAPL()
		if err != nil {
			return 0, 0, errors.Wrap(err, "failed to get power usage")
		}
		energy = energyDif.Package0()
	}

	return duration, energy, nil
}
