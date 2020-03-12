// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbferrors

// MTBFErrCode is a code assigned to different error
type MTBFErrCode int

// MTBFErrType indicates where the error is happending
type MTBFErrType int

const (
	//Based on where errors happen, they are put into different types

	// RemoteError is remote test error
	RemoteError MTBFErrType = iota + 1
	// RemoteFatal is remote test fatal error
	RemoteFatal
	// LocalError is local test error
	LocalError
	// LocalFatal is local test fatal error
	LocalFatal
	// ServiceError is error from service
	ServiceError
	// ServiceFatal is fatal error from service
	ServiceFatal
)

// ErrTypeToStr maps errtype to a readable string
var ErrTypeToStr = map[MTBFErrType]string{
	RemoteError:  "RemoteError",
	RemoteFatal:  "RemoteFatal",
	LocalError:   "LocalError",
	LocalFatal:   "LocalFatal",
	ServiceError: "ServiceError",
	ServiceFatal: "ServiceFatal",
}

// Remote Test OS Error
const (
	Err1100 MTBFErrCode = iota + 1100
	Err1149             = 1149
)

// Remote Test OS Fatal
const (
	Err2100 MTBFErrCode = iota + 2100
	Err2149             = 2149
)

// Local Test OS Error
const (
	Err3100 MTBFErrCode = iota + 3100
	Err3149             = 3149
)

// Local Test OS Fatal
const (
	Err4100 MTBFErrCode = iota + 4100
	Err4101
	Err4102
	Err4149 = 4149
)

// Service Test OS Error
const (
	Err5100 MTBFErrCode = iota + 5100
	Err5149             = 5149
)

// Service Test OS Fatal
const (
	Err6100 MTBFErrCode = iota + 6100
	Err6149             = 6149
)

// Remote Test Chrome Error
const (
	Err1200 MTBFErrCode = iota + 1200
	Err1249             = 1249
)

// Remote Test Chrome Fatal
const (
	Err2200 MTBFErrCode = iota + 2200
	Err2249             = 2249
)

// Local Test Chrome Error
const (
	Err3200 MTBFErrCode = iota + 3200
	Err3201
	Err3202
	Err3203
	Err3204
	Err3205
	Err3206
	Err3207
	Err3208
	Err3209
	Err3210
	Err3211
	Err3212
	Err3213
	Err3214
	Err3249 = 3249
)

// Local Test Chrome Fatal
const (
	Err4200 MTBFErrCode = iota + 4200
	Err4201
	Err4202
	Err4249 = 4249
)

// Service Test Chrome Error
const (
	Err5200 MTBFErrCode = iota + 5200
	Err5249             = 5249
)

// Service Test Chrome Fatal
const (
	Err6200 MTBFErrCode = iota + 6200
	Err6249             = 6249
)

// Remote Test Video Error
const (
	Err1300 MTBFErrCode = iota + 1300
	Err1349             = 1349
)

// Remote Test Video Fatal
const (
	Err2300 MTBFErrCode = iota + 2300
	Err2349             = 2349
)

// Local Test Video Error
const (
	Err3300 MTBFErrCode = iota + 3300
	Err3301
	Err3302
	Err3303
	Err3304
	Err3305
	Err3306
	Err3307
	Err3308
	Err3309
	Err3310
	Err3311
	Err3312
	Err3313
	Err3314
	Err3315
	Err3316
	Err3317
	Err3318
	Err3319
	Err3320
	Err3321
	Err3322
	Err3323
	Err3324
	Err3349 = 3349
)

// Local Test Video Fatal
const (
	Err4300 MTBFErrCode = iota + 4300
	Err4301
	Err4302
	Err4303
	Err4304
	Err4305
	Err4306
	Err4307
	Err4308
	Err4309
	Err4310
	Err4311
	Err4312
	Err4313
	Err4314
	Err4349 = 4349
)

// Service Test Video Error
const (
	Err5300 MTBFErrCode = iota + 5300
	Err5349             = 5349
)

// Service Test Video Fatal
const (
	Err6300 MTBFErrCode = iota + 6300
	Err6349             = 6349
)

// Remote Test Audio Error
const (
	Err1400 MTBFErrCode = iota + 1400
	Err1449             = 1449
)

// Remote Test Audio Fatal
const (
	Err2400 MTBFErrCode = iota + 2400
	Err2449             = 2449
)

// Local Test Audio Error
const (
	Err3400 MTBFErrCode = iota + 3400
	Err3401
	Err3402
	Err3403
	Err3404
	Err3405
	Err3406
	Err3407
	Err3408
	Err3409
	Err3410
	Err3411
	Err3412
	Err3413
	Err3414
	Err3415
	Err3416
	Err3417
	Err3449 = 3449
)

// Local Test Audio Fatal
const (
	Err4400 MTBFErrCode = iota + 4400
	Err4449             = 4449
)

// Service Test Audio Error
const (
	Err5400 MTBFErrCode = iota + 5400
	Err5449             = 5449
)

// Service Test Audio Fatal
const (
	Err6400 MTBFErrCode = iota + 6400
	Err6449             = 6449
)

// Remote Test Bluetooth Error
const (
	Err1500 MTBFErrCode = iota + 1500
	Err1549             = 1549
)

// Remote Test Bluetooth Fatal
const (
	Err2500 MTBFErrCode = iota + 2500
	Err2549             = 2549
)

// Local Test Bluetooth Error
const (
	Err3500 MTBFErrCode = iota + 3500
	Err3501
	Err3502
	Err3503
	Err3504
	Err3505
	Err3506
	Err3507
	Err3508
	Err3509
	Err3510
	Err3511
	Err3512
	Err3549 = 3549
)

// Local Test Bluetooth Fatal
const (
	Err4500 MTBFErrCode = iota + 4500
	Err4549             = 4549
)

// Service Test Bluetooth Error
const (
	Err5500 MTBFErrCode = iota + 5500
	Err5549             = 5549
)

// Service Test Bluetooth Fatal
const (
	Err6500 MTBFErrCode = iota + 6500
	Err6549             = 6549
)

// Remote Test Ethernet Error
const (
	Err1600 MTBFErrCode = iota + 1600
	Err1649             = 1649
)

// Remote Test Ethernet Fatal
const (
	Err2600 MTBFErrCode = iota + 2600
	Err2649             = 2649
)

// Local Test Ethernet Error
const (
	Err3600 MTBFErrCode = iota + 3600
	Err3649             = 3649
)

// Local Test Ethernet Fatal
const (
	Err4600 MTBFErrCode = iota + 4600
	Err4649             = 4649
)

// Service Test Ethernet Error
const (
	Err5600 MTBFErrCode = iota + 5600
	Err5649             = 5649
)

// Service Test Ethernet Fatal
const (
	Err6600 MTBFErrCode = iota + 6600
	Err6649             = 6649
)

// Remote Test Wifi Error
const (
	Err1650 MTBFErrCode = iota + 1650
	Err1609             = 1699
)

// Remote Test Wifi Fatal
const (
	Err2650 MTBFErrCode = iota + 2650
	Err2699             = 2699
)

// Local Test Wifi Error
const (
	Err3650 MTBFErrCode = iota + 3650
	Err3651
	Err3652
	Err3653
	Err3654
	Err3655
	Err3656
	Err3699 = 3699
)

// Local Test Wifi Fatal
const (
	Err4650 MTBFErrCode = iota + 4650
	Err4651
	Err4652
	Err4653
	Err4654
	Err4655
	Err4656
	Err4657
	Err4658
	Err4699 = 4699
)

// Service Test Wifi Error
const (
	Err5650 MTBFErrCode = iota + 5650
	Err5699             = 5699
)

// Service Test Wifi Fatal
const (
	Err6650 MTBFErrCode = iota + 6650
	Err6699             = 6699
)

// Remote Test Camera Error
const (
	Err1700 MTBFErrCode = iota + 1700
	Err1749             = 1749
)

// Remote Test Camera Fatal
const (
	Err2700 MTBFErrCode = iota + 2700
	Err2749             = 2749
)

// Local Test Camera Error
const (
	Err3700 MTBFErrCode = iota + 3700
	Err3749             = 3749
)

// Local Test Camera Fatal
const (
	Err4700 MTBFErrCode = iota + 4700
	Err4701
	Err4702
	Err4703
	Err4704
	Err4705
	Err4706
	Err4707
	Err4708
	Err4709
	Err4710
	Err4711
	Err4712
	Err4713
	Err4714
	Err4715
	Err4716
	Err4717
	Err4718
	Err4749 = 4749
)

// Service Test Camera Error
const (
	Err5700 MTBFErrCode = iota + 5700
	Err5749             = 5749
)

// Service Test Camera Fatal
const (
	Err6700 MTBFErrCode = iota + 6700
	Err6749             = 6749
)

// Remote Test ARC++ System Error
const (
	Err1900 MTBFErrCode = iota + 1900
	Err1949             = 1949
)

// Remote Test ARC++ System Fatal
const (
	Err2900 MTBFErrCode = iota + 2900
	Err2949             = 2949
)

// Local Test ARC++ System Error
const (
	Err3900 MTBFErrCode = iota + 3900
	Err3949             = 3949
)

// Local Test ARC++ System Fatal
const (
	Err4900 MTBFErrCode = iota + 4900
	Err4949             = 4949
)

// Service Test ARC++ System Error
const (
	Err5900 MTBFErrCode = iota + 5900
	Err5949             = 5949
)

// Service Test ARC++ System Fatal
const (
	Err6900 MTBFErrCode = iota + 6900
	Err6949             = 6949
)

// Remote Test ARC++ APP Error
const (
	Err1950 MTBFErrCode = iota + 1950
	Err1909             = 1999
)

// Remote Test ARC++ APP Fatal
const (
	Err2950 MTBFErrCode = iota + 2950
	Err2999             = 2999
)

// Local Test ARC++ APP Error
const (
	Err3950 MTBFErrCode = iota + 3650
	Err3999             = 3699
)

// Local Test ARC++ APP Fatal
const (
	Err4950 MTBFErrCode = iota + 4650
	Err4999             = 4699
)

// Service Test ARC++ APP Error
const (
	Err5950 MTBFErrCode = iota + 5650
	Err5999             = 5699
)

// Service Test ARC++ APP Fatal
const (
	Err6950 MTBFErrCode = iota + 6950
	Err6999             = 6999
)

var errCodeToString = map[MTBFErrCode]string{
	Err1100: `Remote Test OS Error 100`,
	Err1149: `Remote TEst OS Error parameter: %s`,
	Err1900: `Failed to close APK: %s`,
	Err2650: `Remote Test WiFi download failed!`,
	Err3100: `Cannot read "%s" property from yaml configs`,
	Err3200: `Failed to open %s url`,
	Err3201: `Failed to execute javascript code snippet`,
	Err3202: `Failed join AppRtc room`,
	Err3203: `Failed to get histograms data`,
	Err3204: `Failed to open test API connection`,
	Err3205: `Failed to open app: %s`,
	Err3206: `Failed to open %s folder`,
	Err3207: `Failed to close app: %s`,
	Err3208: `Timed out for waiting "%s" element render`,
	Err3209: `Failed to click "%s" in Files App`,
	Err3210: `Failed to get keyboard controller`,
	Err3211: `Failed to press "%s" key`,
	Err3212: `Failed to add terminal listener`,
	Err3213: `crosh terminal error`,
	Err3214: `Failed to join hangout URL=%s`,
	Err3300: `Cannot play video: %s`,
	Err3301: `Unexpected histogram bucket count: %s`,
	Err3302: `Corrupted video file could be played`,
	Err3303: `Unexpected video play behavior`,
	Err3304: `Failed to copy the test audio to %s`,
	Err3305: `The video %s does not play`,
	Err3306: `Failed to play/pause Video Player`,
	Err3307: `OpenStatsForNerd failed`,
	Err3308: `Get video aspect ratio failed`,
	Err3309: `Video frame is not 16:9 nor 4:3: %d x %d`,
	Err3310: `Verify Pause and resume failed`,
	Err3311: `Verify fast forward failed`,
	Err3312: `Verify fast rewind failed`,
	Err3313: `Pause %s video failed`,
	Err3314: `Toggle video to full screen failed`,
	Err3315: `Toggle video to exit full screen failed`,
	Err3316: `Get frame drops failed`,
	Err3317: `Video has frame drops(%d)`,
	Err3318: `Change video quality to %s failed`,
	Err3319: `VerifyRandomSeeking failed`,
	Err3320: `Youtube did not automatic pause`,
	Err3321: `Audio did not automatic pause`,
	Err3322: `Histogram verification failed due to %v`,
	Err3323: `Failed to run %v`,
	Err3324: `%v failed`,
	Err3400: `Corrupted audio file could be played`,
	Err3401: `Unexpected audio play behavior`,
	Err3402: `Failed to play/pause Audio Player`,
	Err3403: `Failed to continue playing Audio Player`,
	Err3404: `Failed to pause Audio Player`,
	Err3405: `Failed to get system sound level`,
	Err3406: `Failed to verify Audio Player is paused`,
	Err3407: `Failed to create audio player struct`,

	Err3408: `No message received`,
	Err3409: `Verification failed, volume is %d`,
	Err3410: `Audio player isn't pausing(Current time: %s, previous time: %s): elapsed time is less than %f second.`,
	Err3411: `Getting audio player playing time failed.`,
	Err3412: `Failed to get operation system sound volume level.`,
	Err3413: `Failed to set operation system sound volume level.`,
	Err3414: `Failed to get operation system sound mute level.`,
	Err3415: `Failed to set operation system sound mute level.`,
	Err3416: `Audio player isn't playing forward(Current time: %s, previous time: %s): elapsed time is less than %f second.`,
	Err3417: `Input volume value is not a number(%s).`,

	Err3500: `Bluetooth setting error!`,
	Err3501: `Failed to enter bt_console`,
	Err3502: `bt_console error command=%v`,
	Err3503: `Failed to turn on bluetooth`,
	Err3504: `Failed to turn off bluetooth`,
	Err3505: `Failed to wait of bluetooth status to be %v`,
	Err3506: `Failed to connect to bluetooth device. deviceName=%v`,
	Err3507: `Failed to get bluetooth device status. deviceName=%v`,
	Err3508: `Bluetooth device is not connected. deviceName=%v`,
	Err3509: `Bluetooth scanning doesn't work.`,
	Err3510: `Failed to connect to BT device in bt_console. address=%v`,
	Err3511: `Bluetooth device is not in A2DP mode! deviceName=%v`,
	Err3512: `Bluetooth device is not in HSP mode! deviceName=%v`,
	Err3651: `Failed to get GUID of NIC. isWiFi=%v apName=%v`,
	Err3652: `Failed to get available WiFi AP list`,
	Err3653: `Failed to enable WiFi apName=%v`,
	Err3654: `Failed to disable WiFi apName=%v`,
	Err3655: `WiFi is not enabled!`,
	Err3656: `WiFi is not disabled!`,
	Err3700: `Failed in %v()`,
	Err4100: `MBTF re-login failed`,
	Err4101: `Failed to open crosh`,
	Err4102: `Test variable not found! varName=%v`,
	Err4200: `Failed to initialize a new chrome`,
	Err4201: `Failed to create a CDP connection with chrome`,
	Err4202: `Failed to create a CDP connection with target %v`,
	Err4301: `Failed to get histogram(%v)`,
	Err4302: `Histogram(%v) has no bucket data`,
	Err4303: `Failed to open dailymotion url`,
	Err4304: `VerifyPlaying failed`,
	Err4305: `ChangeQuality to %v failed`,
	Err4306: `Failed to set up benchmark mode`,
	Err4307: `Failed to wait for CPU to become idle`,
	Err4308: `Failed to set values for verbose logging`,
	Err4309: `Failed to parse test log`,
	Err4310: `Failed to create %s`,
	Err4311: `Failed to copy %s`,
	Err4312: `Faile to open %s url`,
	Err4313: `Waiting for document loading failed`,
	Err4314: `Timed out waiting for playing element`,
	Err4650: `WiFi fatal error! AP Name=%s`,
	Err4651: `Failed to get WiFi AP status through cdp. apName=%s`,
	Err4652: `WiFi Password Error. AP Name=%s`,
	Err4653: `Cannot browse internet through WiFi. AP Name=%s`,
	Err4654: `Failed to check if WiFi enabled in settings.`,
	Err4655: `Failed to download file URL=%s`,
	Err4656: `Failed to enable WiFi network`,
	Err4657: `Failed to disable WiFi network`,
	Err4658: `Failed to run WiFi sub cases of test case %v`,
	Err4700: `Failed to open CCA`,
	Err4701: `Preview is inactive after launching App`,
	Err4702: `Can't get number of cameras`,
	Err4703: `Failed to recognize device mode`,
	Err4704: `Check facing failed`,
	Err4705: `Switch camera failed`,
	Err4706: `Check switch button failed`,
	Err4707: `No camera found`,
	Err4708: `Failed to switch to video mode`,
	Err4709: `Preview is inactive after switch to video mode`,
	Err4710: `Failed to run tests through all cameras`,
	Err4711: `Failed to go to gallery`,
	Err4712: `Failed to restart CCA`,
	Err4713: `Failed to switch to portrait mode`,
	Err4714: `Failed to get app state`,
	Err4715: `Mode selector didn't fallback to photo mode`,
	Err4716: `Check portrait button failed`,
	Err4717: `Failed to start cros-camera`,
	Err4718: `Failed to create Test API connection`,
}
