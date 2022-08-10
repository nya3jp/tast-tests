// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cujrecorder

import (
	"chromiumos/tast/common/perf"
)

// AshCommonMetricConfigs returns common metrics metrics which are required
// to be collected by CUJ tests from the Ash process only.
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
		NewCustomMetricConfig("Apps.StateTransition.Drag.PresentationTime.MaxLatency.TabletMode", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Apps.StateTransition.Drag.PresentationTime.TabletMode", "ms", perf.SmallerIsBetter),

		// Smoothness.
		NewCustomMetricConfig("Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("Graphics.Smoothness.MaxPercentDroppedFrames_1sWindow", "percent", perf.SmallerIsBetter),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenAllApps"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.EnterFullscreenSearch"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.FadeInOverview"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.FadeOutOverview"),
		NewSmoothnessMetricConfig("Apps.HomeLauncherTransition.AnimationSmoothness.HideLauncherForWindow"),
		NewSmoothnessMetricConfig("Apps.PaginationTransition.AnimationSmoothness.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.EnterOverview"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.ExitOverview"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FadeInOverview"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FadeOutOverview"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FullscreenAllApps.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.FullscreenSearch.ClamshellMode"),
		NewSmoothnessMetricConfig("Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode"),
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
		NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Snap"),

		// Media Quality.
		NewCustomMetricConfig("Cras.FetchDelayMilliSeconds", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Cras.UnderrunsPerDevice", "count", perf.SmallerIsBetter),

		// ARC App Kill Metrics
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LMKD.CachedCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LMKD.ForegroundCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LMKD.PerceptibleCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.Pressure.CachedCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.Pressure.ForegroundCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.Pressure.PerceptibleCount10Minutes", "apps", perf.SmallerIsBetter),
		NewCustomMetricConfig("Arc.App.LowMemoryKills.LinuxOOMCount10Minutes", "apps", perf.SmallerIsBetter),

		// Tab Discard Metrics
		NewCustomMetricConfig("Discarding.DailyDiscards.External", "tabs", perf.SmallerIsBetter),
		NewCustomMetricConfig("Discarding.DailyDiscards.Urgent", "tabs", perf.SmallerIsBetter),
		NewCustomMetricConfig("Discarding.DailyReloads.External", "tabs", perf.SmallerIsBetter),
		NewCustomMetricConfig("Discarding.DailyReloads.Urgent", "tabs", perf.SmallerIsBetter),

		// Others to monitor.
		NewCustomMetricConfig("Power.BatteryDischargeRate", "mW", perf.SmallerIsBetter),
		NewLatencyMetricConfig("Ash.DragWindowFromShelf.PresentationTime"),
		NewLatencyMetricConfig("Ash.HotseatTransition.Drag.PresentationTime"),
		NewSmoothnessMetricConfig("Ash.Homescreen.AnimationSmoothness"),
		NewSmoothnessMetricConfig("Ash.SwipeHomeToOverviewGesture"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat"),
		NewSmoothnessMetricConfig("Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat"),
		NewCustomMetricConfig("BootTime.Authenticate", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("BootTime.Chrome", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("BootTime.Firmware", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("BootTime.Kernel", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("BootTime.Login2", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("BootTime.LoginNewUser", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("BootTime.System", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("BootTime.Total2", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("ShutdownTime.BrowserDeleted", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("ShutdownTime.Logout", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("ShutdownTime.Restart", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("ShutdownTime.UIMessageLoopEnded", "ms", perf.SmallerIsBetter),
	}
}

// BrowserCommonMetricConfigs returns common metrics metrics which are
// required to be collected by CUJ tests from the browser process only (Ash or
// Lacros).
func BrowserCommonMetricConfigs() []MetricConfig {
	return []MetricConfig{
		// Responsiveness.
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_NotLoaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.NoSavedFrames_Loaded", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Browser.Tabs.TotalSwitchDuration.WithSavedFrames", "ms", perf.SmallerIsBetter),
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

		// Others to monitor.
		NewCustomMetricConfig("MPArch.RWH_TabSwitchPaintDuration", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("EventLatency.TotalLatency", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("PageLoad.InteractiveTiming.FirstInputDelay4", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("SessionRestore.ForegroundTabFirstPaint4", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Media.Video.Roughness.60fps", "ms", perf.SmallerIsBetter),
		NewCustomMetricConfig("Media.DroppedFrameCount", "count", perf.SmallerIsBetter),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.AllInteractions", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.AllSequences", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Capturer", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.Encoder", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.EncoderQueue", "percent", perf.SmallerIsBetter),
		NewCustomMetricConfig("WebRTC.Video.DroppedFrames.RateLimiter", "percent", perf.SmallerIsBetter),
	}
}

// AnyChromeCommonMetricConfigs returns common metrics metrics which are
// required to be collected by CUJ tests from any Chrome binary running (could
// be from both Ash and Lacros in parallel).
func AnyChromeCommonMetricConfigs() []MetricConfig {
	return []MetricConfig{
		// Smoothness.
		NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Hide"),
	}
}
