// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
)

// MetricConfigs returns metrics which are required to be collected by CUJ tests.
// tconns specifies which test API connections (ash-Chrome or lacros-Chrome) the metric should be collected from.
// Refer to go/speara-metrics.
func MetricConfigs(tconns []*chrome.TestConn) []MetricConfig {
	return []MetricConfig{
		// Responsiveness.
		NewCustomMetricConfigWithTestConns("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter, []int64{100, 1000}, tconns),
		NewCustomMetricConfigWithTestConns("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter, []int64{100, 1000}, tconns),
		NewCustomMetricConfigWithTestConns("Browser.Tabs.TotalSwitchDuration.WithSavedFrames", "ms", perf.SmallerIsBetter, []int64{100, 1000}, tconns),
		NewCustomMetricConfigWithTestConns("Event.Latency.ScrollUpdate.Touch.TimeToScrollUpdateSwapBegin4", "microseconds", perf.SmallerIsBetter, []int64{25000, 150000}, tconns),
		NewCustomMetricConfigWithTestConns("PageLoad.InteractiveTiming.InputDelay3", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Event.Latency.EndToEnd.KeyPress", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}, tconns),
		NewCustomMetricConfigWithTestConns("Event.Latency.EndToEnd.Mouse", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}, tconns),
		NewCustomMetricConfigWithTestConns("Event.Latency.EndToEnd.TouchpadPinch2", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}, tconns),
		NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Ash.TabDrag.PresentationTime.ClamshellMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Ash.TabDrag.PresentationTime.MaxLatency.ClamshellMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Apps.PaginationTransition.DragScroll.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Apps.PaginationTransition.DragScroll.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Apps.StateTransition.Drag.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),
		NewCustomMetricConfigWithTestConns("Apps.StateTransition.Drag.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns),

		// Smoothness.
		NewCustomMetricConfigWithTestConns("Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns),
		NewCustomMetricConfigWithTestConns("Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent", perf.SmallerIsBetter, []int64{20, 50}, tconns),
		NewCustomMetricConfigWithTestConns("Graphics.Smoothness.PercentDroppedFrames.AllInteractions", "percent", perf.SmallerIsBetter, []int64{20, 50}, tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.PaginationTransition.AnimationSmoothness.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FullscreenAllApps.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.EnterOverview", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.ExitOverview", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.Peeking.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.TabletMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.SingleClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.SplitView", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.SingleClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.SplitView", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.TabletMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.MinimizedTabletMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.MinimizedTabletMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.TabletMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Close.TabletMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Rotation.AnimationSmoothness", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.WindowCycleView.AnimationSmoothness.Container", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.WindowCycleView.AnimationSmoothness.Show", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Window.AnimationSmoothness.CrossFade", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Window.AnimationSmoothness.Hide", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Window.AnimationSmoothness.Snap", tconns),
		NewSmoothnessMetricConfigWithTestConns("Chrome.Tabs.AnimationSmoothness.TabLoading", tconns),
		NewSmoothnessMetricConfigWithTestConns("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut", tconns),
		NewSmoothnessMetricConfigWithTestConns("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn", tconns),

		// Browser Render Latency.
		NewCustomMetricConfigWithTestConns("PageLoad.PaintTiming.NavigationToLargestContentfulPaint2", "ms", perf.SmallerIsBetter, []int64{300, 2000}, tconns),
		NewCustomMetricConfigWithTestConns("PageLoad.PaintTiming.NavigationToFirstContentfulPaint", "ms", perf.SmallerIsBetter, []int64{300, 2000}, tconns),
		NewCustomMetricConfigWithTestConns("Browser.Responsiveness.JankyIntervalsPerThirtySeconds", "janks", perf.SmallerIsBetter, []int64{0, 3}, tconns),
		NewCustomMetricConfigWithTestConns("Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks", perf.SmallerIsBetter, []int64{0, 3}, tconns),

		// Startup Latency.
		NewCustomMetricConfigWithTestConns("Startup.FirstWebContents.NonEmptyPaint3", "ms", perf.SmallerIsBetter, []int64{300, 2000}, tconns),

		// Media Quality.
		NewCustomMetricConfigWithTestConns("Cras.FetchDelayMilliSeconds", "ms", perf.SmallerIsBetter, []int64{0, 20}, tconns),
		NewCustomMetricConfigWithTestConns("Cras.UnderrunsPerDevice", "count", perf.SmallerIsBetter, []int64{0, 10}, tconns),

		// Others to monitor.
		NewCustomMetricConfigWithTestConns("MPArch.RWH_TabSwitchPaintDuration", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns),
		NewCustomMetricConfigWithTestConns("PageLoad.InteractiveTiming.FirstInputDelay4", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns),
		NewCustomMetricConfigWithTestConns("SessionRestore.ForegroundTabFirstPaint4", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns),
		NewCustomMetricConfigWithTestConns("Power.BatteryDischargeRate", "mW", perf.SmallerIsBetter, []int64{50, 100}, tconns),
		NewCustomMetricConfigWithTestConns("EventLatency.TotalLatency", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns),
		NewCustomMetricConfigWithTestConns("Graphics.Smoothness.PercentDroppedFrames.AllSequences", "percent", perf.SmallerIsBetter, []int64{20, 50}, tconns),
		NewCustomMetricConfigWithTestConns("Media.Video.Roughness.60fps", "ms", perf.SmallerIsBetter, []int64{20, 50}, tconns),
		NewCustomMetricConfigWithTestConns("Media.DroppedFrameCount", "count", perf.SmallerIsBetter, []int64{10, 40}, tconns),
		NewLatencyMetricConfigWithTestConns("Ash.DragWindowFromShelf.PresentationTime", tconns),
		NewLatencyMetricConfigWithTestConns("Ash.HotseatTransition.Drag.PresentationTime", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.Homescreen.AnimationSmoothness", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.SwipeHomeToOverviewGesture", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FullscreenSearch.ClamshellMode", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.HideLauncherForWindow", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenAllApps", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenSearch", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.FadeInOverview", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.FadeOutOverview", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat", tconns),
		NewSmoothnessMetricConfigWithTestConns("Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FadeInOverview", tconns),
		NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FadeOutOverview", tconns),
		NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.Capturer", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns),
		NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.Encoder", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns),
		NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.EncoderQueue", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns),
		NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.RateLimiter", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns),
	}
}
