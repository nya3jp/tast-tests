// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type cacheMode int

const (
	// Use all caches.
	cacheNormal cacheMode = iota
	// Skip all caches.
	cacheSkipAll
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CachePerf,
		Desc: "Measure benefits of using pre-generated caches for package manager and GMS Core",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      30 * time.Minute,
		Vars:         []string{"arc.CachePerf.username", "arc.CachePerf.password"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// CachePerf steps through opt-in flow 5 times, caches enabled and caches disabled.
// Difference for Play Store shown times in case package manager, GMS Core and other caches
// enabled/disabled are tracked. This difference shows the benefit using the pre-generated
// caches. Results are:
//   * Average time difference for ARC optin in seconds
//   * Average time difference for ARC optin in percents relative to the boot with caches on.
// Optional (in case RAPL interface is available)
//   * Average energy difference required for ARC optin in joules.
//   * Average energy difference required for ARC optin in percents relative to the boot with caches on.
func CachePerf(ctx context.Context, s *testing.State) {
	const (
		// successBootCount is the number of passing ARC boots to collect results.
		successBootCount = 5

		// maxErrorBootCount is the number of maximum allowed boot errors.
		// used for reliability against optin flow flakiness.
		maxErrorBootCount = 1
	)

	normalTime := time.Duration(0)
	skipAllTime := time.Duration(0)
	normalEnergy := float64(0)
	skipAllEnergy := float64(0)
	passedCount := 0
	errorCount := 0

	for passedCount < successBootCount {
		normalTimeIter, skipAllTimeIter, normalEnergyIter, skipAllEneryIter, err := performIteration(ctx, s)
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
		skipAllTime += skipAllTimeIter
		normalEnergy += normalEnergyIter
		skipAllEnergy += skipAllEneryIter
	}

	normalTime /= time.Duration(passedCount)
	skipAllTime /= time.Duration(passedCount)
	normalEnergy /= float64(passedCount)
	skipAllEnergy /= float64(passedCount)
	boost := skipAllTime - normalTime
	percents := 100.0 * boost.Seconds() / normalTime.Seconds()
	s.Logf("Cache time performance: %.1fs (%.1f%%) based on %d passed and %d skipped iterations",
		boost.Seconds(), percents, passedCount, errorCount)

	perfValues := perf.NewValues()
	perfValues.Set(perf.Metric{
		Name:      "boostTime",
		Unit:      "seconds",
		Direction: perf.BiggerIsBetter,
	}, boost.Seconds())
	perfValues.Set(perf.Metric{
		Name:      "boostTimePercents",
		Unit:      "percents",
		Direction: perf.BiggerIsBetter,
	}, percents)

	if normalEnergy != 0 {
		boostEnergy := skipAllEnergy - normalEnergy
		percents = 100.0 * boostEnergy / normalEnergy
		s.Logf("Cache energy performance: %.1f joules (%.1f%%) based on %d passed and %d skipped iterations",
			boostEnergy, percents, passedCount, errorCount)
		perfValues.Set(perf.Metric{
			Name:      "boostEnergy",
			Unit:      "joules",
			Direction: perf.BiggerIsBetter,
		}, boostEnergy)
		perfValues.Set(perf.Metric{
			Name:      "boostEnergyPercents",
			Unit:      "percents",
			Direction: perf.BiggerIsBetter,
		}, percents)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// performIteration performs ARC provisioning in two modes, with and without caches.
// It returns durations spent for each boot.
func performIteration(ctx context.Context, s *testing.State) (normalTime, skipAllTime time.Duration, normalEnergy, skipAllEnergy float64, err error) {
	skipAllTime, skipAllEnergy, err = bootARCCachePerf(ctx, s, cacheSkipAll)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	normalTime, normalEnergy, err = bootARCCachePerf(ctx, s, cacheNormal)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	s.Logf("Iteration done, normal time/energy: %.1fs/%.1fj, no caches time: %.1fs/%.1fj, boost %.1fs/%.1fj",
		normalTime.Seconds(), normalEnergy,
		skipAllTime.Seconds(), skipAllEnergy,
		(skipAllTime - normalTime).Seconds(),
		skipAllEnergy-normalEnergy)
	return normalTime, skipAllTime, normalEnergy, skipAllEnergy, nil
}

// bootARCCachePerf performs Chrome login and boots ARC. It waits for Play Store is shown and
// reports time elapsed from enabling ARC and Play Store is finally shown.
func bootARCCachePerf(ctx context.Context, s *testing.State, mode cacheMode) (time.Duration, float64, error) {
	// TODO(crbug.com/995869): Remove set of flags to disable app sync, PAI, locale sync, Play Store auto-update.
	args := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui", "--ignore-arcvm-dev-conf")

	switch mode {
	case cacheNormal:
	case cacheSkipAll:
		// Disabling package manager cache disables GMS Core cache as well.
		args = append(args, "--arc-packages-cache-mode=skip-copy")
	default:
		return 0, 0, errors.New("invalid cache mode")
	}

	// Drop file caches if any
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		return 0, 0, errors.Wrap(err, "failed to sync caches")
	}
	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0200); err != nil {
		return 0, 0, errors.Wrap(err, "failed to drop caches")
	}

	username := s.RequiredVar("arc.CachePerf.username")
	password := s.RequiredVar("arc.CachePerf.password")

	cr, err := chrome.New(ctx, chrome.ARCSupported(), chrome.RestrictARCCPU(), chrome.GAIALogin(),
		chrome.Auth(username, password, ""), chrome.ExtraArgs(args...))
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to login to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, 0, errors.Wrap(err, "creating test API connection failed")
	}
	defer tconn.Close()

	if err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
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
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		return 0, 0, errors.Wrap(err, "failed to wait Play Store shown")
	}

	duration := time.Now().Sub(startTime)
	energy := float64(0)
	if energyBefore != nil {
		energyDif, err := energyBefore.DiffWithCurrentRAPL()
		if err != nil {
			return 0, 0, errors.Wrap(err, "failed to get power usage")
		}
		energy = energyDif.Total()
	}

	return duration, energy, nil
}
