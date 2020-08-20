// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbferrors

// MTBFErrCode is a code assigned to different errors.
type MTBFErrCode struct {
	code   int
	format string
}

var (
	// Err1000 is the start of MTBF error code.
	Err1000 = MTBFErrCode{1000, `Start of MTBF error code`}
	// IMPORTANT: Please don't define error code less than 1000.

	// General Guideline:
	// - Range 1100 ~ 1199 is for remote test OS errors
	// - Range 1900 ~ 1999 is for remote test ARC errors
	// - Range 2500 ~ 2999 is for remote test fatal errors
	// - Range 3000 ~ 3999 is for local test errors
	// - Range 4000 ~ 4999 is for local test fatal errors
	// other ranges are open to occupy.

	// ARCCloseAPK starts Remote Test ARC++ System Error.
	ARCCloseAPK = MTBFErrCode{1900, `Failed to close APK: %s`}
	// OSRemoteTest starts remote Test OS Error.
	OSRemoteTest = MTBFErrCode{1100, `Remote Test OS Error 100`}
	// NotifyDetachSvr error definition for detach mode.
	NotifyDetachSvr = MTBFErrCode{1101, `Cannot notify wifi.detachStatusServer. URL=%v`}
	// OSNoArcDeviceID error definition.
	OSNoArcDeviceID = MTBFErrCode{1102, `Cannot find DUT Arc device ID`}
	// OSCreateNodeClient error definition.
	OSCreateNodeClient = MTBFErrCode{1103, `Failed on create node client`}
	// OSNodeSendRequest error definition.
	OSNodeSendRequest = MTBFErrCode{1104, `Failed to send request to Node: %s`}
	// OSSDKResultEmpty err definition.
	OSSDKResultEmpty = MTBFErrCode{1105, `SDK with empty case result`}

	// OSTestParam error definition.
	OSTestParam = MTBFErrCode{1149, `Remote Test OS Error parameter: %s`}

	// WIFIDownldR starts remote Test Wifi Fatal.
	WIFIDownldR = MTBFErrCode{2650, `Remote Test WiFi download failed!`}
	// WIFIDownldTimeout error definition.
	WIFIDownldTimeout = MTBFErrCode{2651, `Remote Test WiFi download failed due to timemout!`}

	// ARCRunTast starts remote Test ARC++ System Fatal.
	ARCRunTast = MTBFErrCode{2900, `Failed to run tast %v (last line: %q)`}
	// ARCOpenResult error definition.
	ARCOpenResult = MTBFErrCode{2901, `Couldn't open results.json file`}
	// ARCParseResult error definition.
	ARCParseResult = MTBFErrCode{2902, `Couldn't decode results from %v`}

	// ChromeOpenInURL starts local Test Chrome Error.
	ChromeOpenInURL = MTBFErrCode{3200, `Failed to open internal URL: %s`}
	// ChromeExeJs error definition.
	ChromeExeJs = MTBFErrCode{3201, `Failed to execute javascript code snippet when %s`}
	// ChromeOpenExtURL error definition.
	ChromeOpenExtURL = MTBFErrCode{3202, `Failed to open external URL: %s`}
	// ChromeGetHist error definition.
	ChromeGetHist = MTBFErrCode{3203, `Failed to get histograms data`}
	// ChromeTestConn error definition.
	ChromeTestConn = MTBFErrCode{3204, `Failed to open test API connection`}
	// ChromeOpenApp error definition.
	ChromeOpenApp = MTBFErrCode{3205, `Failed to open app: %s`}
	// ChromeOpenFolder error definition.
	ChromeOpenFolder = MTBFErrCode{3206, `Failed to open %s folder`}
	// ChromeCloseApp error definition.
	ChromeCloseApp = MTBFErrCode{3207, `Failed to close app: %s`}
	// ChromeRenderTime error definition.
	ChromeRenderTime = MTBFErrCode{3208, `Timed out for waiting "%s" element render`}
	// ChromeClickItem error definition.
	ChromeClickItem = MTBFErrCode{3209, `Failed to click "%s" in Files App`}
	// ChromeGetKeyboard error definition.
	ChromeGetKeyboard = MTBFErrCode{3210, `Failed to get keyboard controller`}
	// ChromeKeyPress error definition.
	ChromeKeyPress = MTBFErrCode{3211, `Failed to press "%s" key`}
	// ChromeTermListener error definition.
	ChromeTermListener = MTBFErrCode{3212, `Failed to add terminal listener`}
	// ChromeCrosh error definition.
	ChromeCrosh = MTBFErrCode{3213, `Crosh terminal error`}
	// ChromeJoinHangout error definition.
	ChromeJoinHangout = MTBFErrCode{3214, `Failed to join hangout URL=%s`}
	// ChromeUnknownURL error definition.
	ChromeUnknownURL = MTBFErrCode{3215, `Failed to parse URL: %s`}
	// ChromeNavigate error definition.
	ChromeNavigate = MTBFErrCode{3216, `Failed to navigate URL: %s`}
	// ChromeExistTarget error definition.
	ChromeExistTarget = MTBFErrCode{3217, `Failed to open existed URL: %s`}
	// ChromeClickSystemTray error definition.
	ChromeClickSystemTray = MTBFErrCode{3218, `Failed to click system tray `}
	// ChromeMouse error definition.
	ChromeMouse = MTBFErrCode{3219, `Failed to initialize mouse`}
	// ChromeOpenFileApps error definition.
	ChromeOpenFileApps = MTBFErrCode{3220, `Failed to open Files App`}
	// ChromeOpenAudioPlayer error definition.
	ChromeOpenAudioPlayer = MTBFErrCode{3221, `Failed to open Audio Player`}
	// ChromeOpenVideoPlayer error definition.
	ChromeOpenVideoPlayer = MTBFErrCode{3222, `Failed to open Video Player`}
	// ChromeSleep error definition.
	ChromeSleep = MTBFErrCode{3223, `Failed to sleep`}

	// VideoRoomEmpty error definition.
	VideoRoomEmpty = MTBFErrCode{3303, `AppRtc's room empty, only myself`}
	// VideoCopy error definition.
	VideoCopy = MTBFErrCode{3304, `Failed to copy the test audio to %s`}
	// VideoStatsNerd error definition.
	VideoStatsNerd = MTBFErrCode{3307, `OpenStatsForNerd failed`}
	// VideoGetRatio error definition.
	VideoGetRatio = MTBFErrCode{3308, `Get video aspect ratio failed`}
	// VideoRatio error definition.
	VideoRatio = MTBFErrCode{3309, `Video frame is not 16:9 nor 4:3: %d x %d`}
	// VideoPauseFailed error definition.
	VideoPauseFailed = MTBFErrCode{3313, `Pause %s video failed`}
	// VideoEnterFullSc error definition.
	VideoEnterFullSc = MTBFErrCode{3314, `Toggle video to full screen failed`}
	// VideoExitFullSc error definition.
	VideoExitFullSc = MTBFErrCode{3315, `Toggle video to exit full screen failed`}
	// VideoGetFrmDrop error definition.
	VideoGetFrmDrop = MTBFErrCode{3316, `Get frame drops failed`}
	// VideoFrmDrop error definition.
	VideoFrmDrop = MTBFErrCode{3317, `Video has frame drops(%d)`}
	// VideoChgQuality error definition.
	VideoChgQuality = MTBFErrCode{3318, `Change video quality to %s failed`}
	// VideoHist error definition.
	VideoHist = MTBFErrCode{3322, `Histogram verification failed due to %v`}
	// VideoUTRun error definition.
	VideoUTRun = MTBFErrCode{3323, `Failed to run %v`}
	// VideoUTFailure error definition.
	VideoUTFailure = MTBFErrCode{3324, `%v test case failed`}
	// VideoSeeks error definition.
	VideoSeeks = MTBFErrCode{3325, `Error while seeking, completed %d/%d seeks`}
	// VideoGetTime error definition.
	VideoGetTime = MTBFErrCode{3326, `Getting currentTime from media element failed`}
	// VideoGetSecond error definition.
	VideoGetSecond = MTBFErrCode{3327, `Getting currentTime second time from media element failed`}
	// VideoJumpTo error definition.
	VideoJumpTo = MTBFErrCode{3328, `Fast jump to time is inconsistent, startTime(%f), endTime(%f)`}
	// VideoVeriPause error definition.
	VideoVeriPause = MTBFErrCode{3329, `Verify pause failed: startTime(%f), endTime(%f)`}
	// VideoNoPlay starts local Test Video Error.
	VideoNoPlay = MTBFErrCode{3330, `Media element did not play(current time: %.2f, previous time: %.2f)`}
	// VideoPlaying error definition.
	VideoPlaying = MTBFErrCode{3331, `Media element isn't playing forward(Current time: %.2f, previous time: %.2f): elapsed time is less than %.2f (%.2f)second`}
	// VideoFastJump error definition.
	VideoFastJump = MTBFErrCode{3332, `Media element fast jumps failed`}
	// VideoPause error definition.
	VideoPause = MTBFErrCode{3333, `Video player isn't pausing(Current time: %d, previous time: %d): elapsed time is less than %f second`}
	// VideoPlay error definition.
	VideoPlay = MTBFErrCode{3334, `Video player isn't playing forward(Current time: %d, previous time: %d): elapsed time is less than %f second`}
	// VideoParseTime error definition.
	VideoParseTime = MTBFErrCode{3335, `Cannot parse time %s`}
	// VideoHistNotGrow error definition.
	VideoHistNotGrow = MTBFErrCode{3336, `Histogram %s value did not grow (before: %d, after: %d)`}
	// VideoHistNotEqual error definition.
	VideoHistNotEqual = MTBFErrCode{3337, `Histogram %s value isn't correct, expected: %d, got: %d (before: %d, after: %d)`}
	// VideoNoHist error definition.
	VideoNoHist = MTBFErrCode{3338, `Cannot get histogram: %s`}
	// VideoZeroBucket error definition.
	VideoZeroBucket = MTBFErrCode{3339, `Histogram %s has no value`}
	// VideoMinorFramedrops error definition.
	VideoMinorFramedrops = MTBFErrCode{3340, `Video has a framedrop, current time(%.2f), framedrop(%d), running time(%.2f)`}
	// VideoMajorFramedrops error definition.
	VideoMajorFramedrops = MTBFErrCode{3341, `Video has a few framedrops, current time(%.2f), framedrop(%d), running time(%.2f)`}
	// VideoSevereFramedrops error definition.
	VideoSevereFramedrops = MTBFErrCode{3342, `Video has a serious framedrop issue, current time(%.2f), framedrop(%d), running time(%.2f)`}
	// VideoNoReadyState error definition.
	VideoNoReadyState = MTBFErrCode{3343, `Cannot play media element after media element action`}
	// VideoGetReadyStateFail error definition.
	VideoGetReadyStateFail = MTBFErrCode{3344, `Cannot get media element ready state`}
	// VideoReadyStatePoll error definition.
	VideoReadyStatePoll = MTBFErrCode{3345, `Polling for media element ready state failed`}
	// VideoWaitPlayButton error definition.
	VideoWaitPlayButton = MTBFErrCode{3346, `Failed to wait video play button`}
	// VideoClickPlayButton error definition.
	VideoClickPlayButton = MTBFErrCode{3347, `Failed to click video play button`}
	// VideoWaitPauseButton error definition.
	VideoWaitPauseButton = MTBFErrCode{3348, `Failed to wait video pause button`}
	// VideoClickPauseButton error definition.
	VideoClickPauseButton = MTBFErrCode{3349, `Failed to click video pause button`}
	// VideoPlayElement error definition.
	VideoPlayElement = MTBFErrCode{3350, `Failed to play element`}
	// VideoPauseElement error definition.
	VideoPauseElement = MTBFErrCode{3351, `Failed to pause element`}
	// VideoOpenSettings error definition.
	VideoOpenSettings = MTBFErrCode{3352, `Failed to open video setting`}
	// VideoWaitAndClick error definition.
	VideoWaitAndClick = MTBFErrCode{3353, `Failed to wait and click %s button`}
	// VideoFailFramesPerSeconds error definition.
	VideoFailFramesPerSeconds = MTBFErrCode{3354, `Cannot get frames per second from youtube`}
	// VideoFailCloseTempDir error definition.
	VideoFailCloseTempDir = MTBFErrCode{3355, `Failed to close temp data directory`}
	// VideoPlayFailed starts local Test Video Error.
	VideoPlayFailed = MTBFErrCode{3356, `Cannot play video: %s`}
	// VideoHistCount error definition.
	VideoHistCount = MTBFErrCode{3357, `Unexpected histogram bucket count: %v`}
	// VideoRoomFull error definition.
	VideoRoomFull = MTBFErrCode{3358, `AppRtc's room full, cannot join`}
	// VideoDecoder error definition.
	VideoDecoder = MTBFErrCode{3359, `Got: %q, Expected: %q`}

	// AudioPlaying error definition.
	AudioPlaying = MTBFErrCode{3403, `Failed to continue playing Audio Player`}
	// AudioGetVolLvl error definition.
	AudioGetVolLvl = MTBFErrCode{3405, `Failed to get system sound level`}
	// AudioMute error definition.
	AudioMute = MTBFErrCode{3406, `Verify failed, system audio is not mute`}
	// AudioPlayer error definition.
	AudioPlayer = MTBFErrCode{3407, `Failed to create audio player struct`}
	// AudioNoMsg error definition.
	AudioNoMsg = MTBFErrCode{3408, `No Audio message received`}
	// AudioVolume error definition.
	AudioVolume = MTBFErrCode{3409, `Audio volume verification failed (volume is %d, expected volume is %d)`}
	// AudioPause error definition.
	AudioPause = MTBFErrCode{3410, `Failed to verify Audio Player is paused for %.2f second (current time: %d, previous time: %d)`}
	// AudioPlayTime error definition.
	AudioPlayTime = MTBFErrCode{3411, `Getting audio player playing time failed`}
	// AudioGetOSVol error definition.
	AudioGetOSVol = MTBFErrCode{3412, `Failed to get operation system sound volume level`}
	// AudioSetOSVol error definition.
	AudioSetOSVol = MTBFErrCode{3413, `Failed to set operation system sound volume level`}
	// AudioGetOSMute error definition.
	AudioGetOSMute = MTBFErrCode{3414, `Failed to get operation system sound mute level`}
	// AudioSetOSMute error definition.
	AudioSetOSMute = MTBFErrCode{3415, `Failed to set operation system sound mute level`}
	// AudioPlayFwd error definition.
	AudioPlayFwd = MTBFErrCode{3416, `Audio player isn't playing forward (current time: %d, previous time: %d): elapsed time is less than %.2f second`}
	// AudioInputVol error definition.
	AudioInputVol = MTBFErrCode{3417, `Input volume value is not a number(%s)`}
	// AudioChgVol error definition.
	AudioChgVol = MTBFErrCode{3418, `Audio volume change verification failed, system audio did not change volume (volume is %d, expected volume is %d)`}
	// AudioWaitPauseButton error definition.
	AudioWaitPauseButton = MTBFErrCode{3419, `Failed to wait audio pause button`}
	// AudioClickPauseButton error definition.
	AudioClickPauseButton = MTBFErrCode{3420, `Failed to click audio pause button`}
	// AudioWaitPlayButton error definition.
	AudioWaitPlayButton = MTBFErrCode{3421, `Failed to wait audio play button`}
	// AudioClickPlayButton error definition.
	AudioClickPlayButton = MTBFErrCode{3422, `Failed to click audio play button`}
	// AudioClickCloseButton error definition.
	AudioClickCloseButton = MTBFErrCode{3424, `Failed to click close button`}

	// BTSetting starts Local Test Bluetooth Error.
	BTSetting = MTBFErrCode{6500, `Bluetooth setting error!`}
	// BTEnterCLI error definition.
	BTEnterCLI = MTBFErrCode{6501, `Failed to enter bt_console`}
	// BTCnslCmd error definition.
	BTCnslCmd = MTBFErrCode{6502, `bt_console error command=%v`}
	// BTTurnOn error definition.
	BTTurnOn = MTBFErrCode{6503, `Failed to turn on bluetooth`}
	// BTTurnOff error definition.
	BTTurnOff = MTBFErrCode{6504, `Failed to turn off bluetooth`}
	// BTWaitStatus error definition.
	BTWaitStatus = MTBFErrCode{6505, `Failed to wait for bluetooth status to be %v`}
	// BTConnect error definition.
	BTConnect = MTBFErrCode{6506, `Failed to connect to bluetooth device. deviceName=%v`}
	// BTGetStatus error definition.
	BTGetStatus = MTBFErrCode{6507, `Failed to get bluetooth device status. deviceName=%v`}
	// BTConnected error definition.
	BTConnected = MTBFErrCode{6508, `Bluetooth device is not connected. deviceName=%v`}
	// BTScan error definition.
	BTScan = MTBFErrCode{6509, `Bluetooth scanning doesn't work`}
	// BTCnslConn error definition.
	BTCnslConn = MTBFErrCode{6510, `Failed to connect to BT device in bt_console. address=%v`}
	// BTNotA2DP error definition.
	BTNotA2DP = MTBFErrCode{6511, `Bluetooth device is not in A2DP mode! deviceName=%v`}
	// BTNotHSP error definition.
	BTNotHSP = MTBFErrCode{6512, `Bluetooth device is not in HSP mode! deviceName=%v`}
	// BTA2DPNeeded error definition.
	BTA2DPNeeded = MTBFErrCode{6513, `Bluetooth A2DP device is needed in test case %v`}
	// BTHIDNeeded error definition.
	BTHIDNeeded = MTBFErrCode{6514, `Bluetooth HID device is needed in test case %v`}
	// BTConnectFailed error definition.
	BTConnectFailed = MTBFErrCode{6515, `Bluetooth device (%v) connect failed`}
	// BTServiceNotReady error definition.
	BTServiceNotReady = MTBFErrCode{6516, `Bluetooth (bluez) service not ready`}
	// BTBluezConnError error definition.
	BTBluezConnError = MTBFErrCode{6517, `Failed to connect due to BluezError. address=%v`}
	// BTGetAddress error definition.
	BTGetAddress = MTBFErrCode{6518, `Failed to get bluetooth device address. deviceName=%v`}

	// WIFIGuid starts Local Test Wifi Error.
	WIFIGuid = MTBFErrCode{3651, `Failed to get GUID of NIC. isWiFi=%v apName=%v`}
	// WIFIAPlist error definition.
	WIFIAPlist = MTBFErrCode{3652, `Failed to get available WiFi AP list`}
	// WIFIEnable error definition.
	WIFIEnable = MTBFErrCode{3653, `Failed to enable WiFi. WiFi status is %v`}
	// WIFIDisable error definition.
	WIFIDisable = MTBFErrCode{3654, `Failed to disable WiFi. WiFi status is %v`}
	// WIFIEnabled error definition.
	WIFIEnabled = MTBFErrCode{3655, `WiFi is not enabled!`}
	// WIFIDisabled error definition.
	WIFIDisabled = MTBFErrCode{3656, `WiFi is not disabled!`}
	// WIFIForgetAll error definition.
	WIFIForgetAll = MTBFErrCode{3657, `Failed to disconnect and forget all WiFi APs`}
	// WIFIGetStrInfo error definition.
	WIFIGetStrInfo = MTBFErrCode{3658, `Failed to get WiFi strength info of WiFi AP %v`}

	// APIInvoke starts Local Test Allion API Error.
	APIInvoke = MTBFErrCode{3750, `Failed to invoke Allion API url: %v`}
	// APIUSBCtrl error definition.
	APIUSBCtrl = MTBFErrCode{3751, `Failed to control USB through Allion API. deviceId=%v option=%v resultCode=%v resultTxt=%v`}
	// APIEthCtl error definition.
	APIEthCtl = MTBFErrCode{3752, `Failed to control ethernet through Allion API. deviceId=%v option=%v resultCode=%v resultTxt=%v`}
	// APIWiFiChgStrength error definition.
	APIWiFiChgStrength = MTBFErrCode{3753, `Failed to mannually change WiFi strength through Allion API. strength=%v response=%v`}
	// APIWiFiSetStrength error definition.
	APIWiFiSetStrength = MTBFErrCode{3754, `Failed to set WiFi strength to auto through Allion API. SSID=%v min-strength=%v max-strength=%v resultCode=%v resultTxt=%v`}
	// AttenAPIInvoke starts Local Test Allion API Error.
	AttenAPIInvoke = MTBFErrCode{3755, `Failed to invoke Attenuator API URL: %v. statusCode=%v, responseText=%v`}
	// AttenAPISetStr starts Local Test Allion API Error.
	AttenAPISetStr = MTBFErrCode{3756, `Failed to mannually change WiFi strength through attenuator API. strength=%v`}

	// OSOpenCrosh starts Local Test OS Fatal.
	OSOpenCrosh = MTBFErrCode{4151, `Failed to open crosh`}
	// OSVarRead error definition.
	OSVarRead = MTBFErrCode{4152, `Test variable not found! varName=%v`}
	// OSHostname error definition.
	OSHostname = MTBFErrCode{4153, `Cannot get hostname of DUT`}

	// ChromeInit starts Local Test Chrome Fatal.
	ChromeInit = MTBFErrCode{4200, `Failed to initialize a new chrome`}
	// ChromeCDPConn error definition.
	ChromeCDPConn = MTBFErrCode{4201, `Failed to create a CDP connection with chrome`}
	// ChromeCDPTgt error definition.
	ChromeCDPTgt = MTBFErrCode{4202, `Failed to create a CDP connection with target %v`}
	// ChromeInst error definition.
	ChromeInst = MTBFErrCode{4203, `No chrome instance is available`}
	// ChromeArcLogin error definition.
	ChromeArcLogin = MTBFErrCode{4204, `Failed to do MTBFLogin`}
	// ChromeLogin error definition.
	ChromeLogin = MTBFErrCode{4205, `Login error`}

	// VideoGetHist starts Local Test Video Fatal.
	VideoGetHist = MTBFErrCode{4301, `Failed to get histogram(%v)`}
	// VideoBenchmark error definition.
	VideoBenchmark = MTBFErrCode{4306, `Failed to set up benchmark mode`}
	// VideoCPUIdle error definition.
	VideoCPUIdle = MTBFErrCode{4307, `Failed to wait for CPU to become idle`}
	// VideoLogging error definition.
	VideoLogging = MTBFErrCode{4308, `Failed to set values for verbose logging`}
	// VideoParseLog error definition.
	VideoParseLog = MTBFErrCode{4309, `Failed to parse test log`}
	// VideoCreate error definition.
	VideoCreate = MTBFErrCode{4310, `Failed to create %s`}
	// VideoDocLoad error definition.
	VideoDocLoad = MTBFErrCode{4313, `Waiting for document loading failed`}
	// VideoPlayingEle error definition.
	VideoPlayingEle = MTBFErrCode{4314, `Timed out waiting for playing element`}
	// VideoCPUMeasure error definition.
	VideoCPUMeasure = MTBFErrCode{4315, `Failed to measure CPU usage %v`}
	// VideoDecodeRun error definition.
	VideoDecodeRun = MTBFErrCode{4316, `Decoder did not run long enough for measuring CPU usage`}
	// VideoSubCaseArg err definition.
	VideoSubCaseArg = MTBFErrCode{4317, `%s`}
	// VideoUnknownArg err definition.
	VideoUnknownArg = MTBFErrCode{4318, `Has a unknown argument from previous subcase`}
	// VideoAppRtcRoomName err definition.
	VideoAppRtcRoomName = MTBFErrCode{4319, `Failed on generate a room name`}

	// WIFIFatal starts Local Test Wifi Fatal.
	WIFIFatal = MTBFErrCode{4650, `WiFi fatal error! AP Name=%s`}
	// WIFIGetStat error definition.
	WIFIGetStat = MTBFErrCode{4651, `Failed to get WiFi AP status through cdp. apName=%s`}
	// WIFIPasswd error definition.
	WIFIPasswd = MTBFErrCode{4652, `WiFi Password Error. AP Name=%s`}
	// WIFIInternet error definition.
	WIFIInternet = MTBFErrCode{4653, `Cannot browse internet through WiFi. AP Name=%s`}
	// WIFISetting error definition.
	WIFISetting = MTBFErrCode{4654, `Failed to check if WiFi enabled in settings`}
	// WIFIDownld error definition.
	WIFIDownld = MTBFErrCode{4655, `Failed to download file URL=%s`}
	// WIFIEnableF error definition.
	WIFIEnableF = MTBFErrCode{4656, `Failed to enable WiFi network`}
	// WIFIDisableF error definition.
	WIFIDisableF = MTBFErrCode{4657, `Failed to disable WiFi network`}
	// WIFIDtchPollSubCase error definition.
	WIFIDtchPollSubCase = MTBFErrCode{4658, `WiFi detached sub case polling failed! caseId=%v`}
	// WIFIURLAccess error definition.
	WIFIURLAccess = MTBFErrCode{4659, `WiFi cannot access the URL: %v responseCode: %v`}
	// WIFISleep error definition.
	WIFISleep = MTBFErrCode{4660, `WiFi case sleep failed!`}
	// WFIFDisable error definition.
	WFIFDisable = MTBFErrCode{4661, `Failed to disable WiFi`}

	// CmrOpenCCA starts Local Test Camera Fatal from 4700
	CmrOpenCCA = MTBFErrCode{4700, `Failed to open CCA`}
	// CmrInact error definition.
	CmrInact = MTBFErrCode{4701, `Preview is inactive after launching App`}
	// CmrNumber error definition.
	CmrNumber = MTBFErrCode{4702, `Failed to get number of cameras`}
	// CmrDevMode error definition.
	CmrDevMode = MTBFErrCode{4703, `Failed to recognize device mode`}
	// CmrFacing error definition.
	CmrFacing = MTBFErrCode{4704, `Check facing failed`}
	// CmrSwitch error definition.
	CmrSwitch = MTBFErrCode{4705, `Switch camera failed`}
	// CmrSwitchBtn error definition.
	CmrSwitchBtn = MTBFErrCode{4706, `Check switch button failed`}
	// CmrNotFound error definition.
	CmrNotFound = MTBFErrCode{4707, `No camera found`}
	// CmrVideoRecord error definition.
	CmrVideoRecord = MTBFErrCode{4708, `Failed to record video`}
	// CmrVideoMode error definition.
	CmrVideoMode = MTBFErrCode{4709, `Failed to switch to video mode`}
	// CmrInactVd error definition.
	CmrInactVd = MTBFErrCode{4710, `Preview is inactive after switching to video mode`}
	// CmrTestsAll error definition.
	CmrTestsAll = MTBFErrCode{4711, `Failed to run tests through all cameras`}
	// CmrGallery error definition.
	CmrGallery = MTBFErrCode{4712, `Failed to go to gallery`}
	// CmrGalleryPlay error definition.
	CmrGalleryPlay = MTBFErrCode{4713, `Failed to play video from gallery`}
	// CmrGalleryClose error definition.
	CmrGalleryClose = MTBFErrCode{4714, `Failed to close gallery`}
	// CmrRstCCA error definition.
	CmrRstCCA = MTBFErrCode{4715, `Failed to restart CCA`}
	// CmrPortrait error definition.
	CmrPortrait = MTBFErrCode{4716, `Failed to switch to portrait mode`}
	// CmrAppState error definition.
	CmrAppState = MTBFErrCode{4717, `Failed to get app state`}
	// CmrFallBack error definition.
	CmrFallBack = MTBFErrCode{4718, `Mode selector didn't fallback to photo mode`}
	// CmrPortraitBtn error definition.
	CmrPortraitBtn = MTBFErrCode{4719, `Check portrait button failed`}
	// CmrStart error definition.
	CmrStart = MTBFErrCode{4720, `Failed to start cros-camera`}
	// CmrAPIConn error definition.
	CmrAPIConn = MTBFErrCode{4721, `Failed to create Test API connection`}
	// CmrTakePhoto error definition.
	CmrTakePhoto = MTBFErrCode{4722, `Failed to take photo`}
	// CmrCameraNum error definition.
	CmrCameraNum = MTBFErrCode{4723, `Number of cameras isn't correct`}
	// SwitchUSBRelay error definition.
	SwitchUSBRelay = MTBFErrCode{4724, `Switch USB relay failed`}
	// KbInit error definition.
	KbInit = MTBFErrCode{4725, `Keyboard emulator initialize failed`}
	// SegmentationFault error definition.
	SegmentationFault = MTBFErrCode{4726, `Failed to run %v`}
	// CmrChromeCrashed error definition.
	CmrChromeCrashed = MTBFErrCode{4727, `Failed to open CCA`}

	// GRPCDialFail starts GRPC error code.
	GRPCDialFail = MTBFErrCode{5000, `RPC Dial failed`}
	// GRPCPrePrepare error definition.
	GRPCPrePrepare = MTBFErrCode{5001, `GRPC PrePrepare failed`}
	// GRPCFileNotFound error definition.
	GRPCFileNotFound = MTBFErrCode{5002, `File [%s] not found`}
	// GRPCTransportClosing error definition for transport is closing.
	GRPCTransportClosing = MTBFErrCode{5003, `GRPC Transport Closing`}
	// GRPCBase64Decode error definition for base64 decode failed.
	GRPCBase64Decode = MTBFErrCode{5004, `Base64 decode error`}
	// GRPCOutDirNotSet error definition for no OutDir.
	GRPCOutDirNotSet = MTBFErrCode{5005, `No Decode`}
	// GRPCCreateFileErr error definition for file create.
	GRPCCreateFileErr = MTBFErrCode{5006, `Create File Error [%s]`}
	// GRPCWriteFileErr error definition for file write.
	GRPCWriteFileErr = MTBFErrCode{5007, `Write File Error [%s]`}
	// GRPCLoginARC error definition.
	GRPCLoginARC = MTBFErrCode{5008, `GRPC ARC Login failed`}
	// GRPCOutDir error definition.
	GRPCOutDir = MTBFErrCode{5009, `GRPC output dir creation failed`}
	// GRPCLoginClose error definition.
	GRPCLoginClose = MTBFErrCode{5010, `GRPC Login session close failed`}
	// GRPCTakeScreenshot error definition.
	GRPCTakeScreenshot = MTBFErrCode{5011, `Failed to take screenshot`}
	// GRPCScreenshot error definition.
	GRPCScreenshot = MTBFErrCode{5012, `Failed to save screenshot`}

	// CatsNoDUTName starts CATS error code.
	CatsNoDUTName = MTBFErrCode{6000, `The host name of this DUT is empty`}
	// CatsNoQueryURL error definition.
	CatsNoQueryURL = MTBFErrCode{6001, `The url for querying device info is empty`}
	// CatsQueryFailure error definition.
	CatsQueryFailure = MTBFErrCode{6002, `Failed to query the device info`}
	// CatsNoNodeIP error definition.
	CatsNoNodeIP = MTBFErrCode{6003, `NODE IP is empty`}
	// CatsNoNodeGrpcPort error definition.
	CatsNoNodeGrpcPort = MTBFErrCode{6003, `NODE gRPC port is empty`}
	// CatsNoDUTID error definition.
	CatsNoDUTID = MTBFErrCode{6004, `Device ID is empty`}
	// CatsNoCaseName error definition.
	CatsNoCaseName = MTBFErrCode{6005, `Case name is empty`}
	// CatsErr6006 error definition.
	CatsErr6006 = MTBFErrCode{6006, `Failed to set up a new catsClient`}
	// CatsErr6007 error definition.
	CatsErr6007 = MTBFErrCode{6007, `Failed to Create Task: %s`}
	// CatsErr6008 error definition.
	CatsErr6008 = MTBFErrCode{6008, `Case execution timeout (%s)-(%s)`}
	// CatsErr6009 error definition.
	CatsErr6009 = MTBFErrCode{6009, `Can not find the task [%s] (%s)-(%s)`}
	// CatsErr6010 error definition.
	CatsErr6010 = MTBFErrCode{6010, `Get status error: %s`}
	// CatsErr6011 error definition.
	CatsErr6011 = MTBFErrCode{6011, `Node Origin error: %s`}
	// CatsErr6012 error definition.
	CatsErr6012 = MTBFErrCode{6012, `Invalid Node Port [%d]`}
	// CatsErr6013 error definition.
	CatsErr6013 = MTBFErrCode{6013, `Node Origin error code [%s] converting fail %s`}
	// CatsErr6014 error definition.
	CatsErr6014 = MTBFErrCode{6014, `Json Unmarshal error: %s`}
	// CatsErr6015 error definition.
	CatsErr6015 = MTBFErrCode{6015, `gRPC connection failed`}
	// CatsErr6016 error definition.
	CatsErr6016 = MTBFErrCode{6016, `Get DUT information failed`}
	// CatsNoNodePort error definition.
	CatsNoNodePort = MTBFErrCode{6017, `NODE port is empty`}
	// CatsReportPath error definition.
	CatsReportPath = MTBFErrCode{6018, `NODE report path is empty`}
	// CatsDeviceID error definition.
	CatsDeviceID = MTBFErrCode{6019, `NODE device id is empty`}
	// CatsParameter error definition.
	CatsParameter = MTBFErrCode{6020, `CATS run parameter failed`}
	// CatsAndroidTest error definition.
	CatsAndroidTest = MTBFErrCode{6021, `Failed to new android test`}
	// CatsErr6099 error definition.
	CatsErr6099 = MTBFErrCode{6099, `Unknown Error`}

	// Err6999 ends mtbf error code definition.
	Err6999 = MTBFErrCode{6999, `End of MTBF error code`}
	// !!Important: Err6999 should be the last MTBF error code.
	// Don't define beyond this number.

	// Err7000 - Err7999 are reserved for CATS testing scripts.

	// EnterCameraApp error definition.
	EnterCameraApp = MTBFErrCode{7001, `Fail to enter "Camera" app`}
	// VoiceRecordApp error definition.
	VoiceRecordApp = MTBFErrCode{7001, `Fail to enter "Voice Recorder" app`}
	// HangoutsApp error definition.
	HangoutsApp = MTBFErrCode{7001, `Fail to enter "Hangouts`}
	// CompHangoutsApp error definition.
	CompHangoutsApp = MTBFErrCode{7001, `Fail to enter "Hangouts" app on companion phone`}
	// YoutubeApp error definition.
	YoutubeApp = MTBFErrCode{7001, `Fail to enter Youtube app. The Youtube app is not in main page`}
	// GoogleNewsApp error definition.
	GoogleNewsApp = MTBFErrCode{7001, `Fail to enter News app`}
	// EnterAppMainPage error definition.
	EnterAppMainPage = MTBFErrCode{7001, `Fail to enter app's main page. Should login or allow related permission. App's package name is %s`}
	// NoConversation error definition.
	NoConversation = MTBFErrCode{7002, `Should exist at least one conversation in Hangouts app on companion phone`}
	// FoundVideoFile error definition.
	FoundVideoFile = MTBFErrCode{7002, `Can't find the target video file`}
	// CannotFindTargetFile error definition.
	CannotFindTargetFile = MTBFErrCode{7002, `Can't find the target audio file`}
	// SetUpGoogleNewsEnv error definition.
	SetUpGoogleNewsEnv = MTBFErrCode{7002, `Please set up env. Save a video on News app`}
	// FoundAudioFile error definition.
	FoundAudioFile = MTBFErrCode{7002, `Can't find the target %s audio file.`}
	// FoundSearchBox error definition.
	FoundSearchBox = MTBFErrCode{7004, `Can't find search box or url bar to input url`}
	// VideoResolutionNotExpected error definition.
	VideoResolutionNotExpected = MTBFErrCode{7004, `The resolution of video played in Facebook app is not changed as expected`}
	// VerifyResolution error definition.
	VerifyResolution = MTBFErrCode{7004, `Can't verify the resolution change by comparing player page sizes. Can't find the player size before or after settings full screen`}
	// VerifyYoutubeResolution error definition.
	VerifyYoutubeResolution = MTBFErrCode{7004, `The resolution of video played in youtube app is not changed as expected`}
	// VerifyYoutubeSeek error definition.
	VerifyYoutubeSeek = MTBFErrCode{7004, `The video is not playing from the seek position`}
	// GetDUTResolution error definition.
	GetDUTResolution = MTBFErrCode{7004, `Can't get the resolution of target dut`}
	// VLCKeepStop error definition.
	VLCKeepStop = MTBFErrCode{7005, `VLC keeps stopping occur`}
	// CameraAppCrash error definition.
	CameraAppCrash = MTBFErrCode{7005, `Camera app has crashed`}
	// GCACamera error definition.
	GCACamera = MTBFErrCode{7007, `Fail to enter GCA camera page`}
	// GoogleNewsCrash error definition.
	GoogleNewsCrash = MTBFErrCode{7008, `Video can't play normally. Google News app may have crashed. Please restart dut to recover`}
	// SendHangoutsMessage error definition.
	SendHangoutsMessage = MTBFErrCode{7009, `The message is not sent successfully. Maybe companion phone connect timeout for network issue`}
	// WithoutJoinButton error definition.
	WithoutJoinButton = MTBFErrCode{7009, `Can't wait join appears for 60s. Maybe the network connection is too weak`}
	// YoutubeTitle error definition.
	YoutubeTitle = MTBFErrCode{7009, `The video title doesn't appear on UI. Maybe youtube connect timeout for network issue`}
	// LostResolutionConfig error definition.
	LostResolutionConfig = MTBFErrCode{7010, `Please set default coordinate for target app in resources/case_var/coordinate.xml case var file`}
	// VLCAppNotPause error definition.
	VLCAppNotPause = MTBFErrCode{7011, `The music in VLC app is not pause after playing audio using ChromeOS built in media player`}
	// GoogleMusicNotPlay error definition.
	GoogleMusicNotPlay = MTBFErrCode{7012, `The music in Google Music app is still not playing. Please check the dut behavior`}
	// SpotifyNotPlay error definition.
	SpotifyNotPlay = MTBFErrCode{7012, `The music in Spotify app is not playing. Please check whether is affected by ad`}
	// YoutubeMusicNotPlay error definition.
	YoutubeMusicNotPlay = MTBFErrCode{7012, `The music is not playing`}
	// FacebookVideoNotPlay error definition.
	FacebookVideoNotPlay = MTBFErrCode{7013, `The video in Facebook app is not playing. Maybe the UI element disappears too fast. Can also check whether save a video to "For Later" collection in FaceBook app`}
	// GoogleNewsVideoNotPlay error definition.
	GoogleNewsVideoNotPlay = MTBFErrCode{7013, `The video in News app is not playing. Maybe the UI element disappears too fast`}
	// CannotPlayFacebookVideo error definition.
	CannotPlayFacebookVideo = MTBFErrCode{7013, `Can't play video in facebook`}
	// CannotPlayYoutubeVideo error definition.
	CannotPlayYoutubeVideo = MTBFErrCode{7013, `The video in Youtube app is not playing. Maybe the UI element disappears too fast`}
	// YoutubeNotPlay error definition.
	YoutubeNotPlay = MTBFErrCode{7014, `The video in Youtube app is not playing after the notification center appears. May be affected by notification center. Or just the UI element disappears too fast`}
	// DisableWifi error definition.
	DisableWifi = MTBFErrCode{7015, `Set wifi to False fail`}
	// EnableWifi error definition.
	EnableWifi = MTBFErrCode{7015, `Set wifi to True fail`}
	// VLCAppNotPlay error definition.
	VLCAppNotPlay = MTBFErrCode{7016, `The music in VLC app is not playing`}
	// VLCAppNotPlayFileTypes err definition.
	VLCAppNotPlayFileTypes = MTBFErrCode{7016, `The music(%s) in VLC app is not playing`}
	// EnterContactPage error definition.
	EnterContactPage = MTBFErrCode{7017, `Fail to enter to contact page. Please check whether the contact name configured in properties file is same as actual google account name. Contact name is: %s.`}
	// EnterConversationPage error definition.
	EnterConversationPage = MTBFErrCode{7017, `Fail to enter conversation page`}
	// CanootJoinCall error definition.
	CanootJoinCall = MTBFErrCode{7017, `Can't join to target video call. Maybe former call doesn't end or the video call is full`}
	// StartRecord error definition.
	StartRecord = MTBFErrCode{7018, `Fail to start record`}
	// CannotVerifyRecord error definition.
	CannotVerifyRecord = MTBFErrCode{7018, `Can't get the needed info to verify 'Recorded audio can play to the full recording length'.`}
	// RecordTimeNotMatch error definition.
	RecordTimeNotMatch = MTBFErrCode{7018, `The play time is not same as total record time`}
	// RecordedAudioNotPlay error definition.
	RecordedAudioNotPlay = MTBFErrCode{7018, `The recorded audio in Voice Recorder app is not playing`}
	// CannotGetRecordedFile error definition.
	CannotGetRecordedFile = MTBFErrCode{7018, `Can't get the recorded audio file name in Voice Recorder app`}
	// VideoSwitchButton error definition.
	VideoSwitchButton = MTBFErrCode{7019, `Can not found "Video Switch" Button. Maybe dut CPU/Memory usage is too high`}
	// PhotoButton error definition.
	PhotoButton = MTBFErrCode{7019, `Can not found "Photo Switch" Button. Maybe dut CPU/Memory usage is too high`}
	// WithoutResolution error definition.
	WithoutResolution = MTBFErrCode{7019, `Can't find the resolution of the picture`}
	// EnterGallery error definition.
	EnterGallery = MTBFErrCode{7019, `Fail to enter gallery. Maybe app crashed or can't find any captured frame or recorded video`}
	// StartRecordVideo error definition.
	StartRecordVideo = MTBFErrCode{7019, `Fail to start recording video`}
	// CanootGetImgOrVideoFileName error definition.
	CanootGetImgOrVideoFileName = MTBFErrCode{7020, `Can't get the image/video file name taken in Camera app`}
)

// CatsErrCode The error for CATS Node.
type CatsErrCode struct {
	*MTBFErrCode
	// TaskID is C-ATS tast id.
	TaskID string
	// TaskRptURL is the C-ATS task Report URL.
	TaskRptURL string
	// CatsNodeOrigCode is the original error returned from C-ATS.
	CatsNodeOrigCode string
}
