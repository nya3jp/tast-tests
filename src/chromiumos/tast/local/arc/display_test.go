// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import "testing"

func TestDumpsysPhysicalDisplayP(t *testing.T) {
	const output = `Display Devices: size=1
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

	got, err := scrapeDensity(0, []byte(output), SDKP)
	if err != nil {
		t.Fatal("scrapeDensity failed: ", err)
	}
	const want = 1.875
	if got != want {
		t.Fatalf("scrapeDensity() = %v; want %v", got, want)
	}
}

func TestDumpsysPhysicalDisplayR(t *testing.T) {
	const output = `Display Devices: size=3
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
    mOverrideDisplayInfo=DisplayInfo{"Built-in Screen", displayId 0, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS, FLAG_TRUSTED, real 3840 x 2160, largest app 3840 x 3840, smallest app 2160 x 2160, appVsyncOff 7500000, presDeadline 12666666, mode 1, defaultMode 1, modes [{id=1, width=3840, height=2160, fps=60.000004}], hdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, minimalPostProcessingSupported false, rotation 0, state ON, type INTERNAL, uniqueId "local:21536137753913600", app 3840 x 2160, density 400 (336.331 x 336.588) dpi, layerStack 0, colorMode 0, supportedColorModes [0], address {port=0, model=0x4c8300d0a7e9}, deviceProductInfo DeviceProductInfo{name=, manufacturerPnpId=SDC, productId=16706, modelYear=null, manufactureDate=ManufactureDate{week=19, year=2019}, relativeAddress=null}, removeMode 0}
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

	got, err := scrapeDensity(0, []byte(output), SDKR)
	if err != nil {
		t.Fatal("scrapeDensity failed: ", err)
	}
	const want = 2.0
	if got != want {
		t.Fatalf("scrapeDensity() = %v; want %v", got, want)
	}
}
