package utils

import "chromiumos/tast/local/bundles/cros/arc/wm"

// youtube
const (
	VideoTitle = "SMPTE Television"
	YouTubeUrl = "https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s"
)

// testing app
const (
	// first app
	SettingsPkg = "com.android.settings"
	SettingsAct = ".Settings"

	// second app
	TestappPkg = wm.Pkg24
	TestappAct = wm.ResizableLandscapeActivity
	TestappApk = wm.APKNameArcWMTestApp24
)
