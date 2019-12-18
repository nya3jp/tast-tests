// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type cacheMode int

const (
	// Use all caches.
	cacheNormal cacheMode = iota
	// Skip only GMS Core cache.
	cacheSkipGMSCore
	// Skip both GMS Core and package manager caches.
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
		SoftwareDeps: []string{"android_both", "chrome", "chrome_internal"},
		Timeout:      8 * time.Minute,
		Vars:         []string{"arc.CachePerf.username", "arc.CachePerf.password"},
	})
}

// CachePerf steps through opt-in flow 3 times, warm-up, cache enabled and cache disabled.
// Warm-up step is excluded and difference for Play Store shown times in case package manager
// and GMS Core caches enabled/disabled are tracked. This difference shows the benefit using
// the pre-generated caches for package manager and GMS Core.
func CachePerf(ctx context.Context, s *testing.State) {
	normalTime, err := bootARCCachePerf(ctx, s, cacheNormal)
	if err != nil {
		s.Fatal("Failed to perform boot: ", err)
	}
	s.Logf("normal cache boot done in %s", normalTime.String())

	s.Log("Cache skip GMS Core boot")
	skipGMSCoreTime, err := bootARCCachePerf(ctx, s, cacheSkipGMSCore)
	if err != nil {
		s.Fatal("Failed to perform boot: ", err)
	}
	s.Logf("Skip GMS Core cache boot done in %s", skipGMSCoreTime.String())

	skipAllTime, err := bootARCCachePerf(ctx, s, cacheSkipAll)
	if err != nil {
		s.Fatal("Failed to perform boot: ", err)
	}
	s.Logf("Skip packages and GMS Core cache boot done in %s", skipAllTime.String())

	cachePerfPackages := skipGMSCoreTime - normalTime
	cachePerfGMSCore := skipAllTime - normalTime - cachePerfPackages

	s.Logf("Cache performance: %s package manager, %s GMS core ms", cachePerfPackages.String(), cachePerfGMSCore.String())

	perfValues := perf.NewValues()
	perfValues.Set(perf.Metric{
		Name:      "packages_manager",
		Unit:      "seconds",
		Direction: perf.BiggerIsBetter,
	}, cachePerfPackages.Seconds())
	perfValues.Set(perf.Metric{
		Name:      "gms_core",
		Unit:      "seconds",
		Direction: perf.BiggerIsBetter,
	}, cachePerfGMSCore.Seconds())

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// bootARCCachePerf performs Chrome login and boots ARC. It waits for Play Store is shown and
// reports time elapsed from enabling ARC and Play Store is finally shown.
func bootARCCachePerf(ctx context.Context, s *testing.State, mode cacheMode) (time.Duration, error) {
	// TODO(crbug.com/995869): Remove set of flags to disable app sync, PAI, locale sync, Play Strore auto-update.
	extraArgs := []string{"--arc-force-show-optin-ui", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"}
	switch mode {
	case cacheNormal:
	case cacheSkipGMSCore:
		extraArgs = append(extraArgs, "--arc-disable-gms-core-cache")
	case cacheSkipAll:
		// Disabling package manager cache disables GMS Core cache as well.
		extraArgs = append(extraArgs, "--arc-packages-cache-mode=skip-copy")
	default:
		return 0, errors.New("invalid cache mode")
	}

	username := s.RequiredVar("arc.CachePerf.username")
	password := s.RequiredVar("arc.CachePerf.password")

	cr, err := chrome.New(ctx, chrome.ARCSupported(), chrome.RestrictARCCPU(), chrome.GAIALogin(),
		chrome.Auth(username, password, ""), chrome.ExtraArgs(extraArgs...))
	if err != nil {
		return 0, errors.Wrap(err, "failed to login to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "creating test API connection failed")
	}
	defer tconn.Close()

	if err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	startTime := time.Now()

	s.Log("Waiting for ARC opt-in flow to complete")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return 0, err
	}

	s.Log("Waiting for Play Store window to be shown")
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		return 0, err
	}

	return time.Now().Sub(startTime), nil
}
