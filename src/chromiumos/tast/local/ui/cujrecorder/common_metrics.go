// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cujrecorder

import (
	"chromiumos/tast/common/perf"
)

// AshCommonMetricConfigs returns SPERA common metrics which are
// required to be collected by CUJ tests from the Ash process only.
func AshCommonMetricConfigs() []MetricConfig {
	return []MetricConfig{
		// Responsiveness.
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Ash.TabDrag.PresentationTime.ClamshellMode", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Ash.TabDrag.PresentationTime.MaxLatency.ClamshellMode", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.SingleWindow", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Ash.SplitViewResize.PresentationTime.MaxLatency.ClamshellMode.WithOverview", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Apps.PaginationTransition.DragScroll.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Apps.PaginationTransition.DragScroll.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter),

		// Smoothness.
		NewCustomMetricConfig("Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent", perf.SmallerIsBetter),
		NewSmoothnessMetricConfig("Apps.PaginationTransition.AnimationSmoothness.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FullscreenAllApps.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Peeking.ClamshellMode"),
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
		NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Snap"),

		// Media Quality.
		NewCustomMetricConfig("Cras.FetchDelayMilliSeconds", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Cras.UnderrunsPerDevice", "count", perf.SmallerIsBetter),

		// Other metrics to monitor.
		NewCustomMetricConfig("Power.BatteryDischargeRate", "mW", perf.SmallerIsBetter),
	}
}

// LacrosCommonMetricConfigs returns SPERA common metrics which are
// required to be collected by CUJ tests from the Lacros process only.
func LacrosCommonMetricConfigs() []MetricConfig {
	return []MetricConfig{
		// Smoothness.
		NewCustomMetricConfig("Chrome.Lacros.Smoothness.PercentDroppedFrames_1sWindow", "percent", perf.SmallerIsBetter),
	}
}

// BrowserCommonMetricConfigs returns SEPRA common metrics which are
// required to be collected by CUJ tests from the browser process only
// (Ash or Lacros).
func BrowserCommonMetricConfigs() []MetricConfig {
	return []MetricConfig{
		// Responsiveness.
		// TODO (b/246634445): Replaced with Browser.Tabs.TabSwitchResult2.*, Browser.Tabs.TotalSwitchDuration2.*, Browser.Tabs.TotalIncompleteSwitchDuration2.* and removed.
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.WithSavedFrames", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TabSwitchResult2.NoSavedFrames_NotLoaded", "result", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TabSwitchResult2.NoSavedFrames_Loaded", "result", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TabSwitchResult2.WithSavedFrames", "result", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration2.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration2.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration2.WithSavedFrames", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalIncompleteSwitchDuration2.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalIncompleteSwitchDuration2.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalIncompleteSwitchDuration2.WithSavedFrames", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Event.Latency.ScrollUpdate.Touch.TimeToScrollUpdateSwapBegin4", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.GestureScrollUpdate.Touchscreen.TotalLatency", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.GestureScrollUpdate.Wheel.TotalLatency", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("PageLoad.InteractiveTiming.InputDelay3", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Event.Latency.EndToEnd.KeyPress", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.KeyPressed.TotalLatency", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("Event.Latency.EndToEnd.Mouse", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.MouseDragged.TotalLatency", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.MousePressed.TotalLatency", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.MouseReleased.TotalLatency", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.MouseWheel.TotalLatency", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("Event.Latency.EndToEnd.TouchpadPinch2", "microseconds", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.GesturePinchUpdate.Touchpad.TotalLatency", "microseconds", perf.SmallerIsBetter),

		// Smoothness.
		NewSmoothnessMetricConfig("Chrome.Tabs.AnimationSmoothness.TabLoading"),
		NewSmoothnessMetricConfig("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut"),
		NewSmoothnessMetricConfig("Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn"),

		// Browser Render Latency.
		NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToLargestContentfulPaint2", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToFirstContentfulPaint", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Responsiveness.JankyIntervalsPerThirtySeconds", "janks", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks", perf.SmallerIsBetter),

		// Startup Latency.
		NewCustomMetricConfig("Startup.FirstWebContents.NonEmptyPaint3", "ms", perf.SmallerIsBetter),

		// Other metrics to monitor.
		NewCustomMetricConfig("EventLatency.TotalLatency", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Media.Video.Roughness.60fps", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.AllInteractions", "percent", perf.SmallerIsBetter),
		// TODO (b/247638726): Replaced with Graphics.Smoothness.PercentDroppedFrames3.AllSequences and removed.
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.AllSequences", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames3.AllSequences", "percent", perf.SmallerIsBetter),
	}
}

// AnyChromeCommonMetricConfigs returns SPERA common metrics which are
// required to be collected by CUJ tests from any Chrome binary running
// (could be from both Ash and Lacros in parallel).
func AnyChromeCommonMetricConfigs() []MetricConfig {
	return []MetricConfig{
		// Smoothness.
		NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Hide"),
	}
}

// WebRTCMetrics returns WebRTC common metrics which are required to be collected by conference CUJ tests.
func WebRTCMetrics() []MetricConfig {
	return []MetricConfig{
		NewCustomMetricConfig("WebRTC.Video.BandwidthLimitedResolutionInPercent", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.BandwidthLimitedResolutionsDisabled", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.CpuLimitedResolutionInPercent", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DecodedFramesPerSecond", "fps", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DecodeTimeInMs", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Capturer", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Encoder", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.EncoderQueue", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Ratelimiter", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Receiver", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.InputFramesPerSecond", "fps", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.NumberResolutionDownswitchesPerMinute", "count_per_minute", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.QualityLimitedResolutionDownscales", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.QualityLimitedResolutionInPercent", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.RenderFramesPerSecond", "fps", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.Screenshare.BandwidthLimitedResolutionInPercent", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.Screenshare.BandwidthLimitedResolutionsDisabled", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.Screenshare.InputFramesPerSecond", "fps", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.Screenshare.QualityLimitedResolutionDownscales", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.Screenshare.QualityLimitedResolutionInPercent", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.Screenshare.SentFramesPerSecond", "fps", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.Screenshare.SentToInputFpsRatioPercent", "percent", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.SentFramesPerSecond", "fps", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.SentToInputFpsRatioPercent", "percent", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.SentWidthInPixels", "pixels", perf.BiggerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.TimeInHdPercentage", "percent", perf.BiggerIsBetter),
	}
}
