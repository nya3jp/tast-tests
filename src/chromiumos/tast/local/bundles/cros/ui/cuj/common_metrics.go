// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/cujrecorder"
)

// NewSmoothnessMetricConfigWithTestConns works like NewSmoothnessMetricConfig
// but allows specifying one or multiple test API conns to pull histogram data.
func NewSmoothnessMetricConfigWithTestConns(histogramName string, tconns []*chrome.TestConn) []cujrecorder.MetricConfig {
	result := make([]cujrecorder.MetricConfig, 0)
	for _, tconn := range tconns {
		result = append(result, cujrecorder.DeprecatedNewSmoothnessMetricConfigWithTestConn(histogramName, tconn))
	}
	return result
}

// NewCustomMetricConfigWithTestConns works like NewCustomMetricConfig but
// allows specifying one or multiple test API connections to pull histogram data.
func NewCustomMetricConfigWithTestConns(histogramName, unit string,
	direction perf.Direction, jankCriteria []int64, tconns []*chrome.TestConn) []cujrecorder.MetricConfig {
	result := make([]cujrecorder.MetricConfig, 0)
	for _, tconn := range tconns {
		result = append(result, cujrecorder.DeprecatedNewCustomMetricConfigWithTestConn(histogramName, unit, direction, jankCriteria, tconn))
	}
	return result
}

// NewLatencyMetricConfigWithTestConns works like NewLatencyMetricConfig but
// allows specifying one or multiple test API conns to pull histogram data.
func NewLatencyMetricConfigWithTestConns(histogramName string, tconns []*chrome.TestConn) []cujrecorder.MetricConfig {
	result := make([]cujrecorder.MetricConfig, 0)
	for _, tconn := range tconns {
		result = append(result, cujrecorder.DeprecatedNewLatencyMetricConfigWithTestConn(histogramName, tconn))
	}
	return result
}

// MetricConfigs returns metrics which are required to be collected by CUJ tests.
// tconns specifies which test API connections (ash-Chrome or lacros-Chrome) the metric should be collected from.
// Refer to go/speara-metrics.
func MetricConfigs(tconns []*chrome.TestConn) []cujrecorder.MetricConfig {
	result := make([]cujrecorder.MetricConfig, 0)

	// Responsiveness.
	result = append(result, NewCustomMetricConfigWithTestConns("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter, []int64{100, 1000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter, []int64{100, 1000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Browser.Tabs.TotalSwitchDuration.WithSavedFrames", "ms", perf.SmallerIsBetter, []int64{100, 1000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Event.Latency.ScrollUpdate.Touch.TimeToScrollUpdateSwapBegin4", "microseconds", perf.SmallerIsBetter, []int64{25000, 150000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("PageLoad.InteractiveTiming.InputDelay3", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Event.Latency.EndToEnd.KeyPress", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Event.Latency.EndToEnd.Mouse", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Event.Latency.EndToEnd.TouchpadPinch2", "microseconds", perf.SmallerIsBetter, []int64{25000, 300000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Ash.TabDrag.PresentationTime.ClamshellMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Ash.TabDrag.PresentationTime.MaxLatency.ClamshellMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Apps.PaginationTransition.DragScroll.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Apps.PaginationTransition.DragScroll.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Apps.StateTransition.Drag.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Apps.StateTransition.Drag.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter, []int64{25, 300}, tconns)...)

	// Smoothness.
	result = append(result, NewCustomMetricConfigWithTestConns("Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent", perf.SmallerIsBetter, []int64{20, 50}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Graphics.Smoothness.PercentDroppedFrames.AllInteractions", "percent", perf.SmallerIsBetter, []int64{20, 50}, tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.PaginationTransition.AnimationSmoothness.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FullscreenAllApps.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.EnterOverview", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.ExitOverview", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.Peeking.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.TabletMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.SingleClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.SplitView", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.SingleClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.SplitView", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.TabletMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.MinimizedTabletMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Enter.MinimizedTabletMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Exit.TabletMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Overview.AnimationSmoothness.Close.TabletMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Rotation.AnimationSmoothness", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.WindowCycleView.AnimationSmoothness.Container", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.WindowCycleView.AnimationSmoothness.Show", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Window.AnimationSmoothness.CrossFade", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Window.AnimationSmoothness.Hide", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Window.AnimationSmoothness.Snap", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Chrome.Tabs.AnimationSmoothness.TabLoading", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn", tconns)...)

	// Browser Render Latency.
	result = append(result, NewCustomMetricConfigWithTestConns("PageLoad.PaintTiming.NavigationToLargestContentfulPaint2", "ms", perf.SmallerIsBetter, []int64{300, 2000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("PageLoad.PaintTiming.NavigationToFirstContentfulPaint", "ms", perf.SmallerIsBetter, []int64{300, 2000}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Browser.Responsiveness.JankyIntervalsPerThirtySeconds", "janks", perf.SmallerIsBetter, []int64{0, 3}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks", perf.SmallerIsBetter, []int64{0, 3}, tconns)...)

	// Startup Latency.
	result = append(result, NewCustomMetricConfigWithTestConns("Startup.FirstWebContents.NonEmptyPaint3", "ms", perf.SmallerIsBetter, []int64{300, 2000}, tconns)...)

	// Media Quality.
	result = append(result, NewCustomMetricConfigWithTestConns("Cras.FetchDelayMilliSeconds", "ms", perf.SmallerIsBetter, []int64{0, 20}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Cras.UnderrunsPerDevice", "count", perf.SmallerIsBetter, []int64{0, 10}, tconns)...)

	// Others to monitor.
	result = append(result, NewCustomMetricConfigWithTestConns("MPArch.RWH_TabSwitchPaintDuration", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("PageLoad.InteractiveTiming.FirstInputDelay4", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("SessionRestore.ForegroundTabFirstPaint4", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Power.BatteryDischargeRate", "mW", perf.SmallerIsBetter, []int64{50, 100}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("EventLatency.TotalLatency", "ms", perf.SmallerIsBetter, []int64{800, 1600}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Graphics.Smoothness.PercentDroppedFrames.AllSequences", "percent", perf.SmallerIsBetter, []int64{20, 50}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Media.Video.Roughness.60fps", "ms", perf.SmallerIsBetter, []int64{20, 50}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("Media.DroppedFrameCount", "count", perf.SmallerIsBetter, []int64{10, 40}, tconns)...)
	result = append(result, NewLatencyMetricConfigWithTestConns("Ash.DragWindowFromShelf.PresentationTime", tconns)...)
	result = append(result, NewLatencyMetricConfigWithTestConns("Ash.HotseatTransition.Drag.PresentationTime", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.Homescreen.AnimationSmoothness", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.SwipeHomeToOverviewGesture", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FullscreenSearch.ClamshellMode", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.HideLauncherForWindow", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenAllApps", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenSearch", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.FadeInOverview", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.HomeLauncherTransition.AnimationSmoothness.FadeOutOverview", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FadeInOverview", tconns)...)
	result = append(result, NewSmoothnessMetricConfigWithTestConns("Apps.StateTransition.AnimationSmoothness.FadeOutOverview", tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.Capturer", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.Encoder", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.EncoderQueue", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns)...)
	result = append(result, NewCustomMetricConfigWithTestConns("WebRTC.Video.DroppedFrames.RateLimiter", "percent", perf.SmallerIsBetter, []int64{50, 80}, tconns)...)
	return result
}
