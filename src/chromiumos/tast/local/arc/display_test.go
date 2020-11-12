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

	got, err := scrapeDensity([]byte(output), SDKP)
	if err != nil {
		t.Fatal("scrapeDensity failed: ", err)
	}
	const want = 1.875
	if got != want {
		t.Fatalf("scrapeDensity() = %v; want %v", got, want)
	}
}

func TestDumpsysPhysicalDisplayR(t *testing.T) {
	const output = `Display Devices: size=2
  DisplayDeviceInfo{"Built-in Screen": uniqueId="local:21536137753913600", 3840 x 2160, modeId 1, defaultModeId 1, supportedModes [{id=1, width=3840, height=2160, fps=60.000004}], colorMode 0, supportedColorModes [0], HdrCapabilities HdrCapabilities{mSupportedHdrTypes=[], mMaxLuminance=500.0, mMaxAverageLuminance=500.0, mMinLuminance=0.0}, allmSupported false, gameContentTypeSupported false, density 320, 336.331 x 336.588 dpi, appVsyncOff 7500000, presDeadline 12666666, touch INTERNAL, rotation 0, type INTERNAL, address {port=0, model=0x4c8300d0a7e9}, deviceProductInfo DeviceProductInfo{name=, manufacturerPnpId=SDC, productId=16706, modelYear=null, manufactureDate=ManufactureDate{week=19, year=2019}, relativeAddress=null}, state ON, FLAG_DEFAULT_DISPLAY, FLAG_ROTATES_WITH_CONTENT, FLAG_SECURE, FLAG_SUPPORTS_PROTECTED_BUFFERS}
    mAdapter=LocalDisplayAdapter
    mUniqueId=local:21536137753913600
    mDisplayToken=android.os.BinderProxy@4b44285
    mCurrentLayerStack=0
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
    mState=ON
    mBrightnessState=0.05
    mBacklight=com.android.server.lights.LightsService$LightImpl@4e308da
    mAllmSupported=false
    mAllmRequested=false
    mGameContentTypeSupported=false
    mGameContentTypeRequested=false
    mDisplayInfo=DisplayInfo{isInternal=true, density=2.0, secure=true, deviceProductInfo=DeviceProductInfo{name=, manufacturerPnpId=SDC, productId=16706, modelYear=null, manufactureDate=ManufactureDate{week=19, year=2019}, relativeAddress=null}}
    mDisplayConfigs=
      DisplayConfig{width=3840, height=2160, xDpi=336.331, yDpi=336.588, refreshRate=60.000004, appVsyncOffsetNanos=7500000, presentationDeadlineNanos=12666666, configGroup=-1}
    mSupportedModes=
      DisplayModeRecord{mMode={id=1, width=3840, height=2160, fps=60.000004}}
    mSupportedColorModes=[0]  DisplayDeviceInfo{"ArcNotificationVirtualDisplay": uniqueId="virtual:com.android.systemui,10056,ArcNotificationVirtualDisplay,0", 1 x 1, modeId 2, defaultModeId 2, supportedModes [{id=2, width=1, height=1, fps=60.0}], colorMode 0, supportedColorModes [0], HdrCapabilities null, allmSupported false, gameContentTypeSupported false, density 320, 320.0 x 320.0 dpi, appVsyncOff 0, presDeadline 16666666, touch NONE, rotation 0, type VIRTUAL, deviceProductInfo null, state ON, owner com.android.systemui (uid 10056), FLAG_PRIVATE, FLAG_NEVER_BLANK, FLAG_OWN_CONTENT_ONLY}
mAdapter=VirtualDisplayAdapter
    mUniqueId=virtual:com.android.systemui,10056,ArcNotificationVirtualDisplay,0
    mDisplayToken=android.os.BinderProxy@3f9610b
    mCurrentLayerStack=1
    mCurrentOrientation=0
    mCurrentLayerStackRect=Rect(0, 0 - 1, 1)
    mCurrentDisplayRect=Rect(0, 0 - 1, 1)
    mCurrentSurface=Surface(name=null)/@0x641dbe8
    mFlags=264
    mDisplayState=UNKNOWN
    mStopped=false
    mDisplayIdToMirror=0`

	got, err := scrapeDensity([]byte(output), SDKR)
	if err != nil {
		t.Fatal("scrapeDensity failed: ", err)
	}
	const want = 2.0
	if got != want {
		t.Fatalf("scrapeDensity() = %v; want %v", got, want)
	}
}
