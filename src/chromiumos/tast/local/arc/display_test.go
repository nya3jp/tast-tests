// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"testing"

	"chromiumos/tast/local/coords"
)

func TestDumpsysPhysicalDisplayP(t *testing.T) {
	got, err := scrapeDensity([]byte(dumpsysDisplayP()), 0, SDKP)
	if err != nil {
		t.Fatal("scrapeDensity failed: ", err)
	}
	const want = 1.875
	if got != want {
		t.Fatalf("scrapeDensity() = %v; want %v", got, want)
	}
}

func TestDumpsysPhysicalDisplayR(t *testing.T) {
	gotDefaultDisp, err := scrapeDensity([]byte(dumpsysDisplayR()), 0, SDKR)
	if err != nil {
		t.Fatal("scrapeDensity failed: ", err)
	}
	gotExternalDisp, err := scrapeDensity([]byte(dumpsysDisplayR()), 2, SDKR)
	if err != nil {
		t.Fatal("scrapeDensity failed: ", err)
	}
	const wantDefaultDisp = 2.0
	const wantExternalDisp = 1.33125
	if gotDefaultDisp != wantDefaultDisp {
		t.Fatalf("scrapeDensity(defaultDisp) = %v; want %v", gotDefaultDisp, wantDefaultDisp)
	}
	if gotExternalDisp != wantExternalDisp {
		t.Fatalf("scrapeDensity(externalDisp) = %v; want %v", gotExternalDisp, wantExternalDisp)
	}
}

func TestDumpsysOverrideDensityDPI(t *testing.T) {
	gotDefaultDisp, err := scrapeOverrideDensityDPI([]byte(dumpsysDisplayR()), 0)
	if err != nil {
		t.Fatal("scrapeOverrideDensityDPI failed: ", err)
	}
	gotExternalDisp, err := scrapeOverrideDensityDPI([]byte(dumpsysDisplayR()), 2)
	if err != nil {
		t.Fatal("scrapeOverrideDensityDPI failed: ", err)
	}
	const (
		wantDefaultDisp  = 400
		wantExternalDisp = 213
	)
	if gotDefaultDisp != wantDefaultDisp {
		t.Fatalf("scrapeOverrideDensityDPI(defaultDisp) = %v; want %v", gotDefaultDisp, wantDefaultDisp)
	}
	if gotExternalDisp != wantExternalDisp {
		t.Fatalf("scrapeOverrideDensityDPI(externalDisp) = %v; want %v", gotExternalDisp, wantExternalDisp)
	}
}

func TestDumpsysDisplaySizeR(t *testing.T) {
	got, err := scrapeDisplaySize([]byte(dumpsysDisplayR()), false, 0, SDKR)
	if err != nil {
		t.Fatal("scrapeDisplaySize failed: ", err)
	}
	want := coords.NewSize(2160, 3840)
	if got != want {
		t.Fatalf("scrapeDisplaySize() = %v; want %v", got, want)
	}
}

func TestDumpsysDisplayStableSizeR(t *testing.T) {
	got, err := scrapeDisplaySize([]byte(dumpsysDisplayR()), true, 0, SDKR)
	if err != nil {
		t.Fatal("scrapeDisplayStableSize failed: ", err)
	}
	want := coords.NewSize(3840, 2160)
	if got != want {
		t.Fatalf("scrapeDisplayStableSize() = %v; want %v", got, want)
	}
}

func TestDumpsysScaleFactorR(t *testing.T) {
	got, err := scrapeScaleFactor([]byte(dumpsysWaylandR()), `local:21536137753913600`)
	if err != nil {
		t.Fatal("scrapeScaleFactor failed: ", err)
	}
	const want = 2.666
	if got != want {
		t.Fatalf("scrapeScaleFactor() = %v; want %v", got, want)
	}
}

func dumpsysDisplayP() string {
	return `Display Devices: size=1
  DisplayDeviceInfo{"Built-in Screen": uniqueId="local:0", 2400 x 1600, modeId 1, defaultModeId 1, supportedModes [{id=1, width=2400, height=1600, fps=60.000004}], colorMode 0, supportedColorModes [0], HdrCapabilities android.view.Display$HdrCapabilities@40f16308, density 300, 300.295 x 301.037 dpi, appVsyncOff 1000000, presDeadline 5666666, touch INTERNAL, rotation 0, type BUILT_IN, state ON, FLAG_DEFAULT_DISPLAY, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS}
    mAdapter=LocalDisplayAdapter
    mUniqueId=local:0
    mDisplayToken=android.os.BinderProxy@fb5029e
    mCurrentLayerStack=0
    mCurrentOrientation=0
    mCurrentLayerStackRect=Rect(0, 0 - 2400, 1600)
    mCurrentDisplayRect=Rect(0, 0 - 2400, 1600)
    mCurrentSurface=null
    mBuiltInDisplayId=0
    mActivePhysIndex=0
    mActiveModeId=1
    mActiveColorMode=0
    mState=ON
    mBrightness=102
    mBacklight=com.android.server.lights.LightsService$LightImpl@b12717f
    mDisplayInfos=
      PhysicalDisplayInfo{2400 x 1600, 60.000004 fps, density 1.875, 300.295 x 301.037 dpi, secure true, appVsyncOffset 1000000, bufferDeadline 5666666}
    mSupportedModes=
      DisplayModeRecord{mMode={id=1, width=2400, height=1600, fps=60.000004}}
    mSupportedColorModes=[0]`
}

func dumpsysDisplayR() string {
	return `Display Devices: size=3
  DisplayDeviceInfo{"Built-in Screen": uniqueId="local:21536137753913600", 3840 x 2160, modeId 1, defaultModeId 1, supportedModes [{id=1, width=3840, height=2160, fps=60.000004}], colorMode 0, supportedColorModes [0], HdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, allmSupported false, gameContentTypeSupported false, density 320, 336.331 x 336.588 dpi, appVsyncOff 7500000, presDeadline 12666666, touch INTERNAL, rotation 0, type INTERNAL, address {port=0, model=0x4c8300d0a7e9}, deviceProductInfo DeviceProductInfo{name=, manufacturerPnpId=SDC, productId=16706, modelYear=null, manufactureDate=ManufactureDate{week=19, year=2019}, relativeAddress=null}, state OFF, FLAG_DEFAULT_DISPLAY, FLAG_ROTATES_WITH_CONTENT, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS}
    mAdapter=LocalDisplayAdapter
    mUniqueId=local:21536137753913600
    mDisplayToken=android.os.BinderProxy@a0c06b2
    mCurrentLayerStack=-1
    mCurrentOrientation=0
    mCurrentLayerStackRect=Rect(0, 0 - 3840, 2160)
    mCurrentDisplayRect=Rect(0, 0 - 3840, 2160)
    mCurrentSurface=null
    mPhysicalDisplayId=21536137753913600
    mDisplayModeSpecs={baseModeId=1 primaryRefreshRateRange=[0 60] appRequestRefreshRateRange=[0 Infinity]}
    mDisplayModeSpecsInvalid=false
    mActiveConfigId=0
    mActiveModeId=1
    mActiveColorMode=0
    mDefaultModeId=1
    mState=OFF
    mBrightnessState=-1.0
    mBacklight=com.android.server.lights.LightsService$LightImpl@d54603
    mAllmSupported=false
    mAllmRequested=false
    mGameContentTypeSupported=false
    mGameContentTypeRequested=false
    mDisplayInfo=DisplayInfo{isInternal=true, density=2.0, secure=true, deviceProductInfo=DeviceProductInfo{name=, manufacturerPnpId=SDC, productId=16706, modelYear=null, manufactureDate=ManufactureDate{week=19, year=2019}, relativeAddress=null}}
    mDisplayConfigs=
      DisplayConfig{width=3840, height=2160, xDpi=336.331, yDpi=336.588, refreshRate=60.000004, appVsyncOffsetNanos=7500000, presentationDeadlineNanos=12666666, configGroup=-1}
    mSupportedModes=
      DisplayModeRecord{mMode={id=1, width=3840, height=2160, fps=60.000004}}
    mSupportedColorModes=[0]  DisplayDeviceInfo{"ArcNotificationVirtualDisplay": uniqueId="virtual:com.android.systemui,10039,ArcNotificationVirtualDisplay,0", 1 x 1, modeId 2, defaultModeId 2, supportedModes [{id=2, width=1, height=1, fps=60.0}], colorMode 0, supportedColorModes [0], HdrCapabilities null, allmSupported false, gameContentTypeSupported false, density 400, 400.0 x 400.0 dpi, appVsyncOff 0, presDeadline 16666666, touch NONE, rotation 0, type VIRTUAL, deviceProductInfo null, state ON, owner com.android.systemui (uid 10039), FLAG_PRIVATE, FLAG_NEVER_BLANK, FLAG_OWN_CONTENT_ONLY}
mAdapter=VirtualDisplayAdapter
    mUniqueId=virtual:com.android.systemui,10039,ArcNotificationVirtualDisplay,0
    mDisplayToken=android.os.BinderProxy@f75e980
    mCurrentLayerStack=1
    mCurrentOrientation=0
    mCurrentLayerStackRect=Rect(0, 0 - 1, 1)
    mCurrentDisplayRect=Rect(0, 0 - 1, 1)
    mCurrentSurface=Surface(name=null)/@0xb599bb9
    mFlags=264
    mDisplayState=UNKNOWN
    mStopped=false
    mDisplayIdToMirror=0
  DisplayDeviceInfo{"HDMI Screen": uniqueId="local:1886094531531010", 1920 x 1080, modeId 3, defaultModeId 3, supportedModes [{id=3, width=1920, height=1080, fps=60.000004}], colorMode 0, supportedColorModes [0], HdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, allmSupported false, gameContentTypeSupported false, density 213, 143.435 x 143.623 dpi, appVsyncOff 7500000, presDeadline 12666666, touch EXTERNAL, rotation 0, type EXTERNAL, address {port=2, model=0x6b3649a9091}, deviceProductInfo DeviceProductInfo{name=ASUS MB16AMT, manufacturerPnpId=AUS, productId=5729, modelYear=null, manufactureDate=ManufactureDate{week=41, year=2019}, relativeAddress=[1, 0, 0, 0]}, state OFF, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS, FLAG_PRESENTATION, FLAG_OWN_CONTENT_ONLY}
    mAdapter=LocalDisplayAdapter
    mUniqueId=local:1886094531531010
    mDisplayToken=android.os.BinderProxy@1fe96c
    mCurrentLayerStack=-1
    mCurrentOrientation=0
    mCurrentLayerStackRect=Rect(0, 0 - 1920, 1080)
    mCurrentDisplayRect=Rect(0, 0 - 1920, 1080)
    mCurrentSurface=null
    mPhysicalDisplayId=1886094531531010
    mDisplayModeSpecs={baseModeId=3 primaryRefreshRateRange=[0 60] appRequestRefreshRateRange=[0 Infinity]}
    mDisplayModeSpecsInvalid=false
    mActiveConfigId=0
    mActiveModeId=3
    mActiveColorMode=0
    mDefaultModeId=3
    mState=OFF
    mBrightnessState=NaN
    mBacklight=null
    mAllmSupported=false
    mAllmRequested=false
    mGameContentTypeSupported=false
    mGameContentTypeRequested=false
    mDisplayInfo=DisplayInfo{isInternal=false, density=1.33125, secure=true, deviceProductInfo=DeviceProductInfo{name=ASUS MB16AMT, manufacturerPnpId=AUS, productId=5729, modelYear=null, manufactureDate=ManufactureDate{week=41, year=2019}, relativeAddress=[1, 0, 0, 0]}}
    mDisplayConfigs=
      DisplayConfig{width=1920, height=1080, xDpi=143.435, yDpi=143.623, refreshRate=60.000004, appVsyncOffsetNanos=7500000, presentationDeadlineNanos=12666666, configGroup=-1}
    mSupportedModes=
      DisplayModeRecord{mMode={id=3, width=1920, height=1080, fps=60.000004}}
    mSupportedColorModes=[0]
Logical Displays: size=3
  Display 0:
mDisplayId=0
    mLayerStack=0
    mHasContent=true
    mDesiredDisplayModeSpecs={baseModeId=1 primaryRefreshRateRange=[0 60] appRequestRefreshRateRange=[0 Infinity]}
    mRequestedColorMode=0
    mDisplayOffset=(0, 0)
    mDisplayScalingDisabled=false
    mPrimaryDisplayDevice=Built-in Screen
    mBaseDisplayInfo=DisplayInfo{"Built-in Screen", displayId 0, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS, FLAG_TRUSTED, real 3840 x 2160, largest app 3840 x 2160, smallest app 3840 x 2160, appVsyncOff 7500000, presDeadline 12666666, mode 1, defaultMode 1, modes [{id=1, width=3840, height=2160, fps=60.000004}], hdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, minimalPostProcessingSupported false, rotation 0, state OFF, type INTERNAL, uniqueId "local:21536137753913600", app 3840 x 2160, density 320 (336.331 x 336.588) dpi, layerStack 0, colorMode 0, supportedColorModes [0], address {port=0, model=0x4c8300d0a7e9}, deviceProductInfo DeviceProductInfo{name=, manufacturerPnpId=SDC, productId=16706, modelYear=null, manufactureDate=ManufactureDate{week=19, year=2019}, relativeAddress=null}, removeMode 0}
    mOverrideDisplayInfo=DisplayInfo{"Built-in Screen", displayId 0, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS, FLAG_TRUSTED, real 2160 x 3840, largest app 3840 x 3840, smallest app 2160 x 2160, appVsyncOff 7500000, presDeadline 12666666, mode 1, defaultMode 1, modes [{id=1, width=3840, height=2160, fps=60.000004}], hdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, minimalPostProcessingSupported false, rotation 0, state ON, type INTERNAL, uniqueId "local:21536137753913600", app 3840 x 2160, density 400 (336.331 x 336.588) dpi, layerStack 0, colorMode 0, supportedColorModes [0], address {port=0, model=0x4c8300d0a7e9}, deviceProductInfo DeviceProductInfo{name=, manufacturerPnpId=SDC, productId=16706, modelYear=null, manufactureDate=ManufactureDate{week=19, year=2019}, relativeAddress=null}, removeMode 0}
    mRequestedMinimalPostProcessing=false
  Display 1:
    mDisplayId=1
    mLayerStack=1
    mHasContent=false
    mDesiredDisplayModeSpecs={baseModeId=2 primaryRefreshRateRange=[0 60] appRequestRefreshRateRange=[0 Infinity]}
    mRequestedColorMode=0
    mDisplayOffset=(0, 0)
    mDisplayScalingDisabled=false
    mPrimaryDisplayDevice=ArcNotificationVirtualDisplay
    mBaseDisplayInfo=DisplayInfo{"ArcNotificationVirtualDisplay", displayId 1, FLAG_PRIVATE, real 1 x 1, largest app 1 x 1, smallest app 1 x 1, appVsyncOff 0, presDeadline 16666666, mode 2, defaultMode 2, modes [{id=2, width=1, height=1, fps=60.0}], hdrCapabilities null, minimalPostProcessingSupported false, rotation 0, state ON, type VIRTUAL, uniqueId "virtual:com.android.systemui,10039,ArcNotificationVirtualDisplay,0", app 1 x 1, density 400 (400.0 x 400.0) dpi, layerStack 1, colorMode 0, supportedColorModes [0], deviceProductInfo null, owner com.android.systemui (uid 10039), removeMode 1}
    mOverrideDisplayInfo=DisplayInfo{"ArcNotificationVirtualDisplay", displayId 1, FLAG_PRIVATE, real 1 x 1, largest app 1 x 1, smallest app 1 x 1, appVsyncOff 0, presDeadline 16666666, mode 2, defaultMode 2, modes [{id=2, width=1, height=1, fps=60.0}], hdrCapabilities null, minimalPostProcessingSupported false, rotation 0, state ON, type VIRTUAL, uniqueId "virtual:com.android.systemui,10039,ArcNotificationVirtualDisplay,0", app 1 x 1, density 400 (400.0 x 400.0) dpi, layerStack 1, colorMode 0, supportedColorModes [0], deviceProductInfo null, owner com.android.systemui (uid 10039), removeMode 1}
    mRequestedMinimalPostProcessing=false
  Display 2:
    mDisplayId=2
    mLayerStack=2
    mHasContent=true
    mDesiredDisplayModeSpecs={baseModeId=3 primaryRefreshRateRange=[0 60] appRequestRefreshRateRange=[0 Infinity]}
    mRequestedColorMode=0
    mDisplayOffset=(0, 0)
    mDisplayScalingDisabled=false
    mPrimaryDisplayDevice=HDMI Screen
    mBaseDisplayInfo=DisplayInfo{"HDMI Screen", displayId 2, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS, FLAG_PRESENTATION, FLAG_TRUSTED, real 1920 x 1080, largest app 1920 x 1080, smallest app 1920 x 1080, appVsyncOff 7500000, presDeadline 12666666, mode 3, defaultMode 3, modes [{id=3, width=1920, height=1080, fps=60.000004}], hdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, minimalPostProcessingSupported false, rotation 0, state OFF, type EXTERNAL, uniqueId "local:1886094531531010", app 1920 x 1080, density 213 (143.435 x 143.623) dpi, layerStack 2, colorMode 0, supportedColorModes [0], address {port=2, model=0x6b3649a9091}, deviceProductInfo DeviceProductInfo{name=ASUS MB16AMT, manufacturerPnpId=AUS, productId=5729, modelYear=null, manufactureDate=ManufactureDate{week=41, year=2019}, relativeAddress=[1, 0, 0, 0]}, removeMode 0}
    mOverrideDisplayInfo=DisplayInfo{"HDMI Screen", displayId 2, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS, FLAG_PRESENTATION, FLAG_TRUSTED, real 1920 x 1080, largest app 1920 x 1920, smallest app 1080 x 1080, appVsyncOff 7500000, presDeadline 12666666, mode 3, defaultMode 3, modes [{id=3, width=1920, height=1080, fps=60.000004}], hdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, minimalPostProcessingSupported false, rotation 0, state ON, type EXTERNAL, uniqueId "local:1886094531531010", app 1920 x 1080, density 213 (143.435 x 143.623) dpi, layerStack 2, colorMode 0, supportedColorModes [0], address {port=2, model=0x6b3649a9091}, deviceProductInfo DeviceProductInfo{name=ASUS MB16AMT, manufacturerPnpId=AUS, productId=5729, modelYear=null, manufactureDate=ManufactureDate{week=41, year=2019}, relativeAddress=[1, 0, 0, 0]}, removeMode 0}
    mRequestedMinimalPostProcessing=false`
}

func dumpsysWaylandR() string {
	return `WaylandGlobals
  Interfaces used by client:
      wl_compositor, version: 3
      wl_data_device_manager, version: 3
      wl_output, version: 3
      wl_seat, version: 6
      wl_shm, version: 1
      wl_subcompositor, version: 1
      wp_viewporter, version: 1
      zaura_shell, version: 8
      zwp_pointer_gestures_v1, version: 1
      zwp_pointer_constraints_v1, version: 1
      zwp_relative_pointer_manager_v1, version: 1
      zcr_alpha_compositing_v1, version: 1
      zcr_gaming_input_v2, version: 2
      zcr_keyboard_configuration_v1, version: 2
      zcr_keyboard_extension_v1, version: 1
      zcr_cursor_shapes_v1, version: 1
      zcr_remote_shell_v1, version: 30
      zcr_secure_output_v1, version: 1
      zcr_stylus_v2, version: 2
      zcr_stylus_tools_v1, version: 1
      zcr_vsync_feedback_v1, version: 1
      zwp_linux_dmabuf_v1, version: 2
      zwp_linux_explicit_synchronization_v1, version: 1
  Client will not use explicit-sync protocol
  Display Layout
    Display 21536134253248512 (SF display 21536137753913600, default scale 2.666, zoom factor 1) [primary]
    Display 1885867361823490 (SF display 1886094531531010, default scale 1, zoom factor 1)
  WaylandLayerManager
    Ignored Tasks
      7 
    External Containers
      Container 0x7989e8dcc2c0
        stylus tool: 0       0 layers: [ ]
        0 visible layers: [ ]
      System Container 0x7989f8cd8bd0 (modal false)
        stylus tool: 0       0 layers: [ ]
        0 visible layers: [ ]
    Tracing state: disabled
      number of entries: 0 (0.00MB / 0.00MB)`
}
