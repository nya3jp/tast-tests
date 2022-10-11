// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cujrecorder

import "chromiumos/tast/common/perf"

// CUJAshCommonMetricConfigs returns metrics which are required to be
// collected by all CUJ tests from the Ash process only. This function
// automatically retrieves all SPERA metrics defined in
// AshCommonMetricConfigs and adds more.
func CUJAshCommonMetricConfigs() []MetricConfig {
	return append(AshCommonMetricConfigs(),
		// Smoothness.
		NewCustomMetricConfig("Graphics.Smoothness.MaxPercentDroppedFrames_1sWindow", "percent", perf.SmallerIsBetter),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenAllApps"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenSearch"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.FadeInOverview"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.FadeOutOverview"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.HideLauncherForWindow"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FullscreenSearch.ClamshellMode"),

		// ARC App Kill Metrics.
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LMKD.CachedCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LMKD.ForegroundCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LMKD.PerceptibleCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LinuxOOMCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.Pressure.CachedCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.Pressure.ForegroundCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.Pressure.PerceptibleCount10Minutes", "apps", perf.SmallerIsBetter),

		// Tab Discard Metrics.
		NewCustomMetricConfig("Discarding.DailyDiscards.External", "tabs", perf.SmallerIsBetter),
		NewCustomMetricConfig("Discarding.DailyDiscards.Urgent", "tabs", perf.SmallerIsBetter),
		NewCustomMetricConfig("Discarding.DailyReloads.External", "tabs", perf.SmallerIsBetter),
		NewCustomMetricConfig("Discarding.DailyReloads.Urgent", "tabs", perf.SmallerIsBetter),

		// Desk Metrics.
		NewCustomMetricConfig("Ash.Desks.AnimationLatency.DeskActivation", "ms", perf.SmallerIsBetter),
		NewSmoothnessMetricConfig("Ash.Desks.AnimationSmoothness.DeskActivation"),

		// Other metrics to monitor.
		NewLatencyMetricConfig("Ash.DragWindowFromShelf.PresentationTime"),
		NewLatencyMetricConfig("Ash.HotseatTransition.Drag.PresentationTime"),
		NewSmoothnessMetricConfig("Ash.Homescreen.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat"),
		NewSmoothnessMetricConfig("Ash.SwipeHomeToOverviewGesture"),
		NewBootAndShutdownCustomMetricConfig("BootTime.Authenticate", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("BootTime.Chrome", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("BootTime.Firmware", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("BootTime.Kernel", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("BootTime.Login2", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("BootTime.LoginNewUser", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("BootTime.System", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("BootTime.Total2", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("ShutdownTime.BrowserDeleted", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("ShutdownTime.Logout", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("ShutdownTime.Restart", "ms", perf.SmallerIsBetter),
		NewBootAndShutdownCustomMetricConfig("ShutdownTime.UIMessageLoopEnded", "ms", perf.SmallerIsBetter),
	)
}

// CUJLacrosCommonMetricConfigs returns metrics which are required to
// be collected by all CUJ tests from the Lacros process only. This
// function automatically retrieves all SPERA metrics defined in
// LacrosCommonMetricConfigs and adds more.
func CUJLacrosCommonMetricConfigs() []MetricConfig {
	return LacrosCommonMetricConfigs()
}

// CUJBrowserCommonMetricConfigs returns metrics which are required to
// be collected by all CUJ tests from the browser process only (Ash or
// Lacros). This function automatically retrieves all SPERA metrics
// defined in BrowserCommonMetricConfigs and adds more.
func CUJBrowserCommonMetricConfigs() []MetricConfig {
	return append(BrowserCommonMetricConfigs(),
		// Other metrics to monitor.
		NewCustomMetricConfig("MPArch.RWH_TabSwitchPaintDuration", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Media.DroppedFrameCount", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("PageLoad.InteractiveTiming.FirstInputDelay4", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("SessionRestore.ForegroundTabFirstPaint4", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Capturer", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Encoder", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.EncoderQueue", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.RateLimiter", "percent", perf.SmallerIsBetter),
	)
}

// CUJAnyChromeCommonMetricConfigs returns metrics which are required to
// be collected by CUJ tests from any Chrome binary running (could be
// from both Ash and Lacros in parallel). This function automatically
// retrieves all SPERA metrics defined in AnyChromeCommonMetricConfigs
// and adds more.
func CUJAnyChromeCommonMetricConfigs() []MetricConfig {
	return AnyChromeCommonMetricConfigs()
}
