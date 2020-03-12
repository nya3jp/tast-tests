// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//golint:ignore

package mtbferrors

// MTBFErrCode is a code assigned to different error
type MTBFErrCode struct {
	code   int
	format string
}

var (
	// Err1000 is the start of MTBF error code
	Err1000 = MTBFErrCode{1000, `Start of MTBF error code`}
	// IMPORTANT: Please don't define error code less than 1000.

	// General Guideline:
	// - Range 1100 ~ 1199 is for remote test OS errors
	// - Range 1900 ~ 1999 is for remote test ARC errors
	// - Range 2500 ~ 2999 is for remote test fatal errors
	// - Range 3000 ~ 3999 is for local test errors
	// - Range 4000 ~ 4999 is for local test fatal errors
	// other ranges are open to occupy

	// ARCCloseAPK starts Remote Test ARC++ System Error
	ARCCloseAPK = MTBFErrCode{1900, `Failed to close APK: %s`}

	// OSRemoteTest starts remote Test OS Error
	OSRemoteTest = MTBFErrCode{1100, `Remote Test OS Error 100`}
	// OSTestParam error definition
	OSTestParam = MTBFErrCode{1149, `Remote Test OS Error parameter: %s`}

	// WIFIDownldR starts remote Test Wifi Fatal
	WIFIDownldR = MTBFErrCode{2650, `Remote Test WiFi download failed!`}
	// WIFIDownldTimeout error definition
	WIFIDownldTimeout = MTBFErrCode{2651, `Remote Test WiFi download failed due to timemout!`}

	// ARCRunTast starts remote Test ARC++ System Fatal
	ARCRunTast = MTBFErrCode{2900, `failed to run tast %v (last line: %q)`}
	// ARCOpenResult error definition
	ARCOpenResult = MTBFErrCode{2901, `couldn't open results.json file`}
	// ARCParseResult error definition
	ARCParseResult = MTBFErrCode{2902, `couldn't decode results from %v`}

	// ChromeOpenInURL starts local Test Chrome Error
	ChromeOpenInURL = MTBFErrCode{3200, `Failed to open internal URL: %s`}
	// ChromeExeJs error definition
	ChromeExeJs = MTBFErrCode{3201, `Failed to execute javascript code snippet`}
	// ChromeOpenExtURL error definition
	ChromeOpenExtURL = MTBFErrCode{3202, `Failed to open external URL: %s`}
	// ChromeGetHist error definition
	ChromeGetHist = MTBFErrCode{3203, `Failed to get histograms data`}
	// ChromeTestConn error definition
	ChromeTestConn = MTBFErrCode{3204, `Failed to open test API connection`}
	// ChromeOpenApp error definition
	ChromeOpenApp = MTBFErrCode{3205, `Failed to open app: %s`}
	// ChromeOpenFolder error definition
	ChromeOpenFolder = MTBFErrCode{3206, `Failed to open %s folder`}
	// ChromeCloseApp error definition
	ChromeCloseApp = MTBFErrCode{3207, `Failed to close app: %s`}
	// ChromeRenderTime error definition
	ChromeRenderTime = MTBFErrCode{3208, `Timed out for waiting "%s" element render`}
	// ChromeClickItem error definition
	ChromeClickItem = MTBFErrCode{3209, `Failed to click "%s" in Files App`}
	// ChromeGetKeyboard error definition
	ChromeGetKeyboard = MTBFErrCode{3210, `Failed to get keyboard controller`}
	// ChromeKeyPress error definition
	ChromeKeyPress = MTBFErrCode{3211, `Failed to press "%s" key`}
	// ChromeTermListener error definition
	ChromeTermListener = MTBFErrCode{3212, `Failed to add terminal listener`}
	// ChromeCrosh error definition
	ChromeCrosh = MTBFErrCode{3213, `crosh terminal error`}
	// ChoromeJoinHangout error definition
	ChoromeJoinHangout = MTBFErrCode{3214, `Failed to join hangout URL=%s`}
	// ChromeUnknownURL error definition
	ChromeUnknownURL = MTBFErrCode{3215, `Failed to parse URL: %s`}
	// ChromeNavigate error definition
	ChromeNavigate = MTBFErrCode{3216, `Failed to navigate URL: %s`}
	// ChromeExistTarget error definition
	ChromeExistTarget = MTBFErrCode{3217, `Failed to open existed URL: %s`}

	// VideoNoPlay starts local Test Video Error
	VideoNoPlay = MTBFErrCode{3300, `Cannot play video: %s`}
	// VideoHistCount error definition
	VideoHistCount = MTBFErrCode{3301, `Unexpected histogram bucket count: %s`}
	// VideoJoinRTC error definition
	VideoJoinRTC = MTBFErrCode{3302, `AppRtc room full, cannot join`}
	// VideoCopy error definition
	VideoCopy = MTBFErrCode{3304, `Failed to copy the test audio to %s`}
	// VideoNotPlay error definition
	VideoNotPlay = MTBFErrCode{3305, `The video %s does not play`}
	// VideoPlayPause error definition
	VideoPlayPause = MTBFErrCode{3306, `Failed to play/pause Video Player`}
	// VideoStatsNerd error definition
	VideoStatsNerd = MTBFErrCode{3307, `OpenStatsForNerd failed`}
	// VideoGetRatio error definition
	VideoGetRatio = MTBFErrCode{3308, `Get video aspect ratio failed`}
	// VideoRatio error definition
	VideoRatio = MTBFErrCode{3309, `Video frame is not 16:9 nor 4:3: %d x %d`}
	// VideoPauseResume error definition
	VideoPauseResume = MTBFErrCode{3310, `Verify Pause and resume failed`}
	// VideoFastFwd error definition
	VideoFastFwd = MTBFErrCode{3311, `Verify fast forward failed`}
	// VideoFastRwd error definition
	VideoFastRwd = MTBFErrCode{3312, `Verify fast rewind failed`}
	// VideoNoPause error definition
	VideoNoPause = MTBFErrCode{3313, `Pause %s video failed`}
	// VideoEnterFullSc error definition
	VideoEnterFullSc = MTBFErrCode{3314, `Toggle video to full screen failed`}
	// VideoExitFullSc error definition
	VideoExitFullSc = MTBFErrCode{3315, `Toggle video to exit full screen failed`}
	// VideoGetFrmDrop error definition
	VideoGetFrmDrop = MTBFErrCode{3316, `Get frame drops failed`}
	// VideoFrmDrop error definition
	VideoFrmDrop = MTBFErrCode{3317, `Video has frame drops(%d)`}
	// VideoChgQuality error definition
	VideoChgQuality = MTBFErrCode{3318, `Change video quality to %s failed`}
	// VideoSeek error definition
	VideoSeek = MTBFErrCode{3319, `VerifyRandomSeeking failed`}
	// VideoYTPause error definition
	VideoYTPause = MTBFErrCode{3320, `Youtube did not automatic pause`}
	// VideoHist error definition
	VideoHist = MTBFErrCode{3322, `Histogram verification failed due to %v`}
	// VideoUTRun error definition
	VideoUTRun = MTBFErrCode{3323, `Failed to run %v`}
	// VideoUTFailure error definition
	VideoUTFailure = MTBFErrCode{3324, `%v failed`}
	// VideoSeeks error definition
	VideoSeeks = MTBFErrCode{3325, `Error while seeking, completed %d/%d seeks.`}
	// VideoGetTime error definition
	VideoGetTime = MTBFErrCode{3326, `Getting currentTime from media element failed.`}
	// VideoGetSecond error definition
	VideoGetSecond = MTBFErrCode{3327, `Getting currentTime second time from media element failed.`}
	// VideoJumpTo error definition
	VideoJumpTo = MTBFErrCode{3328, `Fast jump to time is inconsistent, startTime(%f), endTime(%f).`}
	// VideoVeriPause error definition
	VideoVeriPause = MTBFErrCode{3329, `Verify pause failed: startTime(%f), endTime(%f).`}
	// VideoNotPlay2 error definition
	VideoNotPlay2 = MTBFErrCode{3330, `Media element did not play(current time: %.2f, previous time: %.2f).`}
	// VideoPlaying error definition
	VideoPlaying = MTBFErrCode{3331, `Media element isn't playing forward(Current time: %.2f, previous time: %.2f): elapsed time is less than %.2f (%.2f)second`}
	// VideoFastJump error definition
	VideoFastJump = MTBFErrCode{3332, `Media element fast jumps failed.`}
	// VideoPause error definition
	VideoPause = MTBFErrCode{3333, `Video player isn't pausing(Current time: %d, previous time: %d): elapsed time is less than %f second`}
	// VideoPlay error definition
	VideoPlay = MTBFErrCode{3334, `Video player isn't playing forward(Current time: %d, previous time: %d): elapsed time is less than %f second`}
	// VideoParseTime error definition
	VideoParseTime = MTBFErrCode{3335, `Cannot parse time %s`}

	// AudioPlayPause starts Local Test Audio Error
	AudioPlayPause = MTBFErrCode{3402, `Failed to play/pause Audio Player`}
	// AudioPlaying error definition
	AudioPlaying = MTBFErrCode{3403, `Failed to continue playing Audio Player`}
	// AudioGetVolLvl error definition
	AudioGetVolLvl = MTBFErrCode{3405, `Failed to get system sound level`}
	// AudioMute error definition
	AudioMute = MTBFErrCode{3406, `Verify failed, system audio is not mute.`}
	// AudioPlayer error definition
	AudioPlayer = MTBFErrCode{3407, `Failed to create audio player struct`}
	// AudioNoMsg error definition
	AudioNoMsg = MTBFErrCode{3408, `No message received`}
	// AudioVolume error definition
	AudioVolume = MTBFErrCode{3409, `Verification failed, volume is %d`}
	// AudioPause error definition
	AudioPause = MTBFErrCode{3410, `Failed to verify Audio Player is paused for %.2f second (current time: %.2f, previous time: %.2f)`}
	// AudioPlayTime error definition
	AudioPlayTime = MTBFErrCode{3411, `Getting audio player playing time failed.`}
	// AudioGetOSVol error definition
	AudioGetOSVol = MTBFErrCode{3412, `Failed to get operation system sound volume level.`}
	// AudioSetOSVol error definition
	AudioSetOSVol = MTBFErrCode{3413, `Failed to set operation system sound volume level.`}
	// AudioGetOSMute error definition
	AudioGetOSMute = MTBFErrCode{3414, `Failed to get operation system sound mute level.`}
	// AudioSetOSMute error definition
	AudioSetOSMute = MTBFErrCode{3415, `Failed to set operation system sound mute level.`}
	// AudioPlayFwd error definition
	AudioPlayFwd = MTBFErrCode{3416, `Audio player isn't playing forward (current time: %.2f, previous time: %.2f): elapsed time is less than %.2f second`}
	// AudioInputVol error definition
	AudioInputVol = MTBFErrCode{3417, `Input volume value is not a number(%s).`}
	// AudioChgVol error definition
	AudioChgVol = MTBFErrCode{3418, `Verify failed, system audio does not change volume(%d, %d).`}

	// BTSetting starts Local Test Bluetooth Error
	BTSetting = MTBFErrCode{3500, `Bluetooth setting error!`}
	// BTEnterCLI error definition
	BTEnterCLI = MTBFErrCode{3501, `Failed to enter bt_console`}
	// BTCnslCmd error definition
	BTCnslCmd = MTBFErrCode{3502, `bt_console error command=%v`}
	// BTTurnOn error definition
	BTTurnOn = MTBFErrCode{3503, `Failed to turn on bluetooth`}
	// BTTurnOff error definition
	BTTurnOff = MTBFErrCode{3504, `Failed to turn off bluetooth`}
	// BTWaitStatus error definition
	BTWaitStatus = MTBFErrCode{3505, `Failed to wait of bluetooth status to be %v`}
	// BTConnect error definition
	BTConnect = MTBFErrCode{3506, `Failed to connect to bluetooth device. deviceName=%v`}
	// BTGetStatus error definition
	BTGetStatus = MTBFErrCode{3507, `Failed to get bluetooth device status. deviceName=%v`}
	// BTConnected error definition
	BTConnected = MTBFErrCode{3508, `Bluetooth device is not connected. deviceName=%v`}
	// BTScan error definition
	BTScan = MTBFErrCode{3509, `Bluetooth scanning doesn't work.`}
	// BTCnslConn error definition
	BTCnslConn = MTBFErrCode{3510, `Failed to connect to BT device in bt_console. address=%v`}
	// BTNotA2DP error definition
	BTNotA2DP = MTBFErrCode{3511, `Bluetooth device is not in A2DP mode! deviceName=%v`}
	// BTNotHSP error definition
	BTNotHSP = MTBFErrCode{3512, `Bluetooth device is not in HSP mode! deviceName=%v`}
	// BTA2DPNeeded error definition
	BTA2DPNeeded = MTBFErrCode{3513, `Bluetooth A2DP device is needed in test case %v`}
	// BTHIDNeeded error definition
	BTHIDNeeded = MTBFErrCode{3514, `Bluetooth HID device is needed in test case %v`}

	// WIFIGuid starts Local Test Wifi Error
	WIFIGuid = MTBFErrCode{3651, `Failed to get GUID of NIC. isWiFi=%v apName=%v`}
	// WIFIAPlist error difinition
	WIFIAPlist = MTBFErrCode{3652, `Failed to get available WiFi AP list`}
	// WIFIEnable error difinition
	WIFIEnable = MTBFErrCode{3653, `Failed to enable WiFi. WiFi status is %v`}
	// WIFIDisable error difinition
	WIFIDisable = MTBFErrCode{3654, `Failed to disable WiFi. WiFi status is %v`}
	// WIFIEnabled error difinition
	WIFIEnabled = MTBFErrCode{3655, `WiFi is not enabled!`}
	// WIFIDisabled error difinition
	WIFIDisabled = MTBFErrCode{3656, `WiFi is not disabled!`}

	// APIInvoke starts Local Test Allion API Error
	APIInvoke = MTBFErrCode{3750, `Failed to invoke Allion API url: %v`}
	// APIUSBCtrl error difinition
	APIUSBCtrl = MTBFErrCode{3751, `Failed to control USB through Allion API. deviceId=%v option=%v resultCode=%v resultTxt=%v`}
	// APIEthCtl error difinition
	APIEthCtl = MTBFErrCode{3752, `Failed to control ethernet through Allion API. deviceId=%v option=%v resultCode=%v resultTxt=%v`}
	// APIWiFiChgStrength error difinition
	APIWiFiChgStrength = MTBFErrCode{3753, `Failed to mannually change WiFi strength through Allion API. SSID=%v strength=%v resultCode=%v resultTxt=%v`}
	// APIWiFiSetStrength error difinition
	APIWiFiSetStrength = MTBFErrCode{3754, `Failed to set WiFi strength to auto through Allion API. SSID=%v min-strength=%v max-strength=%v resultCode=%v resultTxt=%v`}

	// OSOpenCrosh starts Local Test OS Fatal
	OSOpenCrosh = MTBFErrCode{4101, `Failed to open crosh`}
	// OSVarRead error difinition
	OSVarRead = MTBFErrCode{4102, `Test variable not found! varName=%v`}
	// OSHostname error difinition
	OSHostname = MTBFErrCode{4103, `Cannot get hostname of DUT`}

	// ChromeInit starts Local Test Chrome Fatal
	ChromeInit = MTBFErrCode{4200, `Failed to initialize a new chrome`}
	// ChromeCDPConn error difinition
	ChromeCDPConn = MTBFErrCode{4201, `Failed to create a CDP connection with chrome`}
	// ChromeCDPTgt error difinition
	ChromeCDPTgt = MTBFErrCode{4202, `Failed to create a CDP connection with target %v`}
	// ChromeInst error difinition
	ChromeInst = MTBFErrCode{4203, `No chrome instance is available`}

	// VideoGetHist starts Local Test Video Fatal
	VideoGetHist = MTBFErrCode{4301, `Failed to get histogram(%v)`}
	// VideoOpenURL error difinition
	VideoOpenURL = MTBFErrCode{4303, `Failed to open dailymotion url`}
	// VideoVeriPlay error difinition
	VideoVeriPlay = MTBFErrCode{4304, `VerifyPlaying failed`}
	// VideoChgQuality2 error difinition
	VideoChgQuality2 = MTBFErrCode{4305, `ChangeQuality to %v failed`}
	// VideoBenchmark error difinition
	VideoBenchmark = MTBFErrCode{4306, `Failed to set up benchmark mode`}
	// VideoCPUIdle error difinition
	VideoCPUIdle = MTBFErrCode{4307, `Failed to wait for CPU to become idle`}
	// VideoLogging error difinition
	VideoLogging = MTBFErrCode{4308, `Failed to set values for verbose logging`}
	// VideoParseLog error difinition
	VideoParseLog = MTBFErrCode{4309, `Failed to parse test log`}
	// VideoCreate error difinition
	VideoCreate = MTBFErrCode{4310, `Failed to create %s`}
	// VideoCopy2 error difinition
	VideoCopy2 = MTBFErrCode{4311, `Failed to copy %s`}
	// VideoOpenURL2 error difinition
	VideoOpenURL2 = MTBFErrCode{4312, `Failed to open %s url`}
	// VideoDocLoad error difinition
	VideoDocLoad = MTBFErrCode{4313, `Waiting for document loading failed`}
	// VideoPlayingEle error difinition
	VideoPlayingEle = MTBFErrCode{4314, `Timed out waiting for playing element`}
	// VideoCPUMeasure error difinition
	VideoCPUMeasure = MTBFErrCode{4315, `Failed to measure CPU usage %v`}
	// VideoDecodeRun error difinition
	VideoDecodeRun = MTBFErrCode{4316, `Decoder did not run long enough for measuring CPU usage`}

	// WIFIFatal starts Local Test Wifi Fatal
	WIFIFatal = MTBFErrCode{4650, `WiFi fatal error! AP Name=%s`}
	// WIFIGetStat error difinition
	WIFIGetStat = MTBFErrCode{4651, `Failed to get WiFi AP status through cdp. apName=%s`}
	// WIFIPasswd error difinition
	WIFIPasswd = MTBFErrCode{4652, `WiFi Password Error. AP Name=%s`}
	// WIFIInternet error difinition
	WIFIInternet = MTBFErrCode{4653, `Cannot browse internet through WiFi. AP Name=%s`}
	// WIFISetting error difinition
	WIFISetting = MTBFErrCode{4654, `Failed to check if WiFi enabled in settings.`}
	// WIFIDownld error difinition
	WIFIDownld = MTBFErrCode{4655, `Failed to download file URL=%s`}
	// WIFIEnableF error difinition
	WIFIEnableF = MTBFErrCode{4656, `Failed to enable WiFi network`}
	// WIFIDisableF error difinition
	WIFIDisableF = MTBFErrCode{4657, `Failed to disable WiFi network`}

	// CmrOpenCCA starts Local Test Camera Fatal from 4700
	CmrOpenCCA = MTBFErrCode{4700, `Failed to open CCA`}
	// CmrInact error difinition
	CmrInact = MTBFErrCode{4701, `Preview is inactive after launching App`}
	// CmrNumber error difinition
	CmrNumber = MTBFErrCode{4702, `Can't get number of cameras`}
	// CmrDevMode error difinition
	CmrDevMode = MTBFErrCode{4703, `Failed to recognize device mode`}
	// CmrFacing error difinition
	CmrFacing = MTBFErrCode{4704, `Check facing failed`}
	// CmrSwitch error difinition
	CmrSwitch = MTBFErrCode{4705, `Switch camera failed`}
	// CmrSwitchBtn error difinition
	CmrSwitchBtn = MTBFErrCode{4706, `Check switch button failed`}
	// CmrNotFound error difinition
	CmrNotFound = MTBFErrCode{4707, `No camera found`}
	// CmrVideoRecord error difinition
	CmrVideoRecord = MTBFErrCode{4708, `Failed to record video`}
	// CmrVideoMode error difinition
	CmrVideoMode = MTBFErrCode{4709, `Failed to switch to video mode`}
	// CmrInactVd error difinition
	CmrInactVd = MTBFErrCode{4710, `Preview is inactive after switch to video mode`}
	// CmrTestsAll error difinition
	CmrTestsAll = MTBFErrCode{4711, `Failed to run tests through all cameras`}
	// CmrGallery error difinition
	CmrGallery = MTBFErrCode{4712, `Failed to go to gallery`}
	// CmrGalleryPlay error difinition
	CmrGalleryPlay = MTBFErrCode{4713, `Failed to play video from gallery`}
	// CmrGalleryClose error difinition
	CmrGalleryClose = MTBFErrCode{4714, `Failed to close gallery`}
	// CmrRstCCA error difinition
	CmrRstCCA = MTBFErrCode{4715, `Failed to restart CCA`}
	// CmrPortrait error difinition
	CmrPortrait = MTBFErrCode{4716, `Failed to switch to portrait mode`}
	// CmrAppState error difinition
	CmrAppState = MTBFErrCode{4717, `Failed to get app state`}
	// CmrFallBack error difinition
	CmrFallBack = MTBFErrCode{4718, `Mode selector didn't fallback to photo mode`}
	// CmrPortraitBtn error difinition
	CmrPortraitBtn = MTBFErrCode{4719, `Check portrait button failed`}
	// CmrStart error difinition
	CmrStart = MTBFErrCode{4720, `Failed to start cros-camera`}
	// CmrAPIConn error difinition
	CmrAPIConn = MTBFErrCode{4721, `Failed to create Test API connection`}
	// CmrTakePhoto error difinition
	CmrTakePhoto = MTBFErrCode{4722, `Failed to take photo`}

	// Err6999 ends mtbf error code definition
	Err6999 = MTBFErrCode{6999, `End of MTBF error code`}
	// !!Important: Err6999 should be the last MTBF error code.
	// Don't define beyond this number.
)
