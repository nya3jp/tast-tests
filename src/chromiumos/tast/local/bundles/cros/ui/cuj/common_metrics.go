// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import "chromiumos/tast/common/perf"

// MetricConfigs returns default metric collection need be monitoring
func MetricConfigs() []MetricConfig {
	configs := []MetricConfig{
		NewCustomMetricConfig("MPArch.RWH_TabSwitchPaintDuration", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.WithSavedFrames", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToLargestContentfulPaint", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("PageLoad.InteractiveTiming.InputDelay3", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("PageLoad.InteractiveTiming.FirstInputDelay4", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("SessionRestore.ForegroundTabFirstPaint4", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("Startup.FirstWebContents.NonEmptyPaint3", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("Power.BatteryDischargeRate", "mW", perf.SmallerIsBetter, []int64{50, 100}),
		NewCustomMetricConfig("Event.Latency.ScrollUpdate.Touch.TimeToScrollUpdateSwapBegin4", "microseconds", perf.SmallerIsBetter, []int64{50000, 80000}),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.AllSequences", "percent", perf.SmallerIsBetter, []int64{50, 20}),
		NewLatencyMetricConfig("Ash.DragWindowFromShelf.PresentationTime"),
		NewLatencyMetricConfig("Ash.HotseatTransition.Drag.PresentationTime"),
		NewLatencyMetricConfig("Net.TCP_Connection_Latency"),
		NewLatencyMetricConfig("Net.HttpTimeToFirstByte"),
		NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Container"),
		NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Show"),
		NewSmoothnessMetricConfig("Ash.Rotation.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Ash.Homescreen.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Ash.SwipeHomeToOverviewGesture"),
	}

	for _, suffix := range []string{"HideLauncherForWindow", "EnterFullscreenAllApps", "EnterFullscreenSearch", "FadeInOverview", "FadeOutOverview"} {
		configs = append(configs, NewSmoothnessMetricConfig(
			"Apps.HomeLauncherTransition.AnimationSmoothness."+suffix))
	}
	for _, state := range []string{"Peeking", "Close", "Half"} {
		configs = append(configs, NewSmoothnessMetricConfig(
			"Apps.StateTransition.AnimationSmoothness."+state+".ClamshellMode"))
	}
	for _, suffix := range []string{"SingleClamshellMode", "ClamshellMode", "TabletMode"} {
		configs = append(configs,
			NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter."+suffix),
			NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit."+suffix),
		)
	}
	for _, suffix := range []string{"TransitionToShownHotseat", "TransitionToExtendedHotseat", "TransitionToHiddenHotseat"} {
		configs = append(configs,
			NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness."+suffix))
	}
	for _, suffix := range []string{"Capturer", "Encoder", "EncoderQueue", "RateLimiter"} {
		configs = append(configs, NewCustomMetricConfig(
			"WebRTC.Video.DroppedFrames."+suffix, "percent", perf.SmallerIsBetter,
			[]int64{50, 80}))
	}

	return configs
}
