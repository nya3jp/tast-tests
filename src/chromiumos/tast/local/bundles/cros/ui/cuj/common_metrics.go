// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import "chromiumos/tast/common/perf"

// MetricConfigs returns default metric collection need to be collected by CUJ tests.
func MetricConfigs() []MetricConfig {
	return []MetricConfig{
		// Responsiveness
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter, []int64{100, 1000}),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter, []int64{100, 1000}),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.WithSavedFrames", "ms", perf.SmallerIsBetter, []int64{100, 1000}),
		NewCustomMetricConfig("Event.Latency.ScrollUpdate.Touch.TimeToScrollUpdateSwapBegin4", "microseconds", perf.SmallerIsBetter, []int64{25000, 150000}),
		NewCustomMetricConfig("PageLoad.InteractiveTiming.InputDelay3", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Event.Latency.EndToEnd.KeyPress", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}),
		NewCustomMetricConfig("Event.Latency.EndToEnd.Mouse", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}),
		NewCustomMetricConfig("Event.Latency.EndToEnd.TouchpadPinch2", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}),
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Ash.TabDrag.PresentationTime.ClamshellMode", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Ash.TabDrag.PresentationTime.MaxLatency.ClamshellMode", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Apps.PaginationTransition.DragScroll.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Apps.PaginationTransition.DragScroll.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Apps.StateTransition.Drag.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}),
		NewCustomMetricConfig("Apps.StateTransition.Drag.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}),

		// Smoothness
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent", perf.SmallerIsBetter, []int64{20, 50}),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.AllInteractions", "percent", perf.SmallerIsBetter, []int64{20, 50}),
		NewSmoothnessMetricConfig("Apps.PaginationTransition.AnimationSmoothness.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FullscreenAllApps.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.EnterOverview"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.ExitOverview"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Peeking.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.TabletMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter.ClamshellMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter.SingleClamshellMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter.SplitView"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit.ClamshellMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit.SingleClamshellMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit.SplitView"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter.TabletMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit.MinimizedTabletMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter.MinimizedTabletMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit.TabletMode"),
		NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Close.TabletMode"),
		NewSmoothnessMetricConfig("Ash.Rotation.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Container"),
		NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Show"),
		NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.CrossFade"),
		NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Hide"),
		NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Snap"),
		NewSmoothnessMetricConfig("Chrome.Tabs.AnimationSmoothness.TabLoading"),
		NewSmoothnessMetricConfig("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut"),
		NewSmoothnessMetricConfig("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn"),

		// Browser Render Latency
		NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToLargestContentfulPaint", "ms", perf.SmallerIsBetter, []int64{300, 2000}),
		NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToFirstContentfulPaint", "ms", perf.SmallerIsBetter, []int64{300, 2000}),
		NewCustomMetricConfig("Browser.Responsiveness.JankyIntervalsPerThirtySeconds", "janks", perf.SmallerIsBetter, []int64{0, 3}),

		// Startup Latency
		NewCustomMetricConfig("Startup.FirstWebContents.NonEmptyPaint3", "ms", perf.SmallerIsBetter, []int64{300, 2000}),

		// Media Quality
		NewCustomMetricConfig("Cras.FetchDelayMilliSeconds", "ms", perf.SmallerIsBetter, []int64{0, 20}),
		NewCustomMetricConfig("Cras.UnderrunsPerDevice", "count", perf.SmallerIsBetter, []int64{0, 10}),

		// Others to monitor
		NewCustomMetricConfig("MPArch.RWH_TabSwitchPaintDuration", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("PageLoad.InteractiveTiming.FirstInputDelay4", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("SessionRestore.ForegroundTabFirstPaint4", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("Power.BatteryDischargeRate", "mW", perf.SmallerIsBetter, []int64{50, 100}),
		NewCustomMetricConfig("EventLatency.TotalLatency", "ms", perf.SmallerIsBetter, []int64{800, 1600}),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.AllSequences", "percent", perf.SmallerIsBetter, []int64{20, 50}),
		NewCustomMetricConfig("Media.Video.Roughness.60fps", "ms", perf.SmallerIsBetter, []int64{20, 50}),
		NewCustomMetricConfig("Media.DroppedFrameCount", "count", perf.SmallerIsBetter, []int64{10, 40}),
		NewLatencyMetricConfig("Ash.DragWindowFromShelf.PresentationTime"),
		NewLatencyMetricConfig("Ash.HotseatTransition.Drag.PresentationTime"),
		NewSmoothnessMetricConfig("Ash.Homescreen.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Ash.SwipeHomeToOverviewGesture"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FullscreenSearch.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.HideLauncherForWindow"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenAllApps"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenSearch"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.FadeInOverview"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.FadeOutOverview"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FadeInOverview"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FadeOutOverview"),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Capturer", "percent", perf.SmallerIsBetter, []int64{50, 80}),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Encoder", "percent", perf.SmallerIsBetter, []int64{50, 80}),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.EncoderQueue", "percent", perf.SmallerIsBetter, []int64{50, 80}),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.RateLimiter", "percent", perf.SmallerIsBetter, []int64{50, 80}),
	}
}
