// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

// chameleondDefaultXMLRPCPort is the default port for the chameleond XMLRPC
// interface.
const chameleondDefaultXMLRPCPort = 9992

// PortType is the human-readable name of a chameleon port's connector type
// name, as returned by Chameleond.GetConnectorType.
//
// For ease of use, known PortType values are included in this package as
// constants prefixed with "PortType".
type PortType string

const (
	// PortTypeDP is the PortType for a DisplayPort chameleon port.
	PortTypeDP PortType = "DP"

	// PortTypeHDMI is the PortType for an HDMI chameleon port.
	PortTypeHDMI PortType = "HDMI"

	// PortTypeVGA is the PortType for a VGA chameleon port.
	PortTypeVGA PortType = "VGA"

	// PortTypeMIC is the PortType for a microphone chameleon port.
	PortTypeMIC PortType = "Mic"

	// PortTypeAnalogAudioLineIn is the PortType for an audio line-in chameleon
	// port.
	PortTypeAnalogAudioLineIn PortType = "LineIn"

	// PortTypeAnalogAudioLineOut is the PortType for an audio line-out chameleon
	// port.
	PortTypeAnalogAudioLineOut PortType = "LineOut"

	// PortTypeUSBAudioIn is the PortType for a USB audio-in chameleon port.
	PortTypeUSBAudioIn PortType = "USBIn"

	// PortTypeUSBAudioOut is the PortType for a USB audio-out chameleon port.
	PortTypeUSBAudioOut PortType = "USBOut"

	// PortTypeUSBKeyboard is the PortType for a USB keyboard chameleon port.
	PortTypeUSBKeyboard PortType = "USBKeyboard"

	// PortTypeUSBTouch is the PortType for a USB touch chameleon port.
	PortTypeUSBTouch PortType = "USBTouch"

	// PortTypeBluetoothHIDKeyboard is the PortType for a bluetooth HID keyboard
	// chameleon port.
	PortTypeBluetoothHIDKeyboard PortType = "bluetooth_hid_keyboard"

	// PortTypeBluetoothHIDGamepad is the PortType for a bluetooth HID gamepad
	// chameleon port.
	PortTypeBluetoothHIDGamepad PortType = "bluetooth_hid_gamepad"

	// PortTypeBluetoothHIDMouse is the PortType for a bluetooth HID mouse
	// chameleon port.
	PortTypeBluetoothHIDMouse PortType = "bluetooth_hid_mouse"

	// PortTypeBluetoothHIDCombo is the PortType for a bluetooth HID combo
	// chameleon port.
	PortTypeBluetoothHIDCombo PortType = "bluetooth_hid_combo"

	// PortTypeBluetoothHIDJoystick is the PortType for a bluetooth HID joystick
	//chameleon port.
	PortTypeBluetoothHIDJoystick PortType = "bluetooth_hid_joystick"

	// PortTypeAVSyncProbe is the PortType for an AV sync probe chameleon port.
	PortTypeAVSyncProbe PortType = "avsync_probe"

	// PortTypeAudioBoard is the PortType for an audio board chameleon port.
	PortTypeAudioBoard PortType = "audio_board"

	// PortTypeMotorBoard is the PortType for a motor board chameleon port.
	PortTypeMotorBoard PortType = "motor_board"

	// PortTypeBluetoothHOGKeyboard is the PortType for a bluetooth HOG keyboard
	// chameleon port.
	PortTypeBluetoothHOGKeyboard PortType = "bluetooth_hog_keyboard"

	// PortTypeBluetoothHOGGamepad is the PortType for a bluetooth HOG gamepad
	// chameleon port.
	PortTypeBluetoothHOGGamepad PortType = "bluetooth_hog_gamepad"

	// PortTypeBluetoothHOGMouse is the PortType for a bluetooth HOG mouse
	// chameleon port.
	PortTypeBluetoothHOGMouse PortType = "bluetooth_hog_mouse"

	// PortTypeBluetoothHOGCombo is the PortType for a bluetooth HOG combo
	// chameleon port.
	PortTypeBluetoothHOGCombo PortType = "bluetooth_hog_combo"

	// PortTypeBluetoothHOGJoystick is the PortType for a bluetooth HOG joystick
	// chameleon port.
	PortTypeBluetoothHOGJoystick PortType = "bluetooth_hog_joystick"

	// PortTypeUSBPrinter is the PortType for a USB printer chameleon port.
	PortTypeUSBPrinter PortType = "usb_printer"

	// PortTypeBluetoothA2DPSink is the PortType for a bluetooth A2DP sink
	// chameleon port.
	PortTypeBluetoothA2DPSink PortType = "bluetooth_a2dp_sink"

	// PortTypeBLEMouse is the PortType for a bluetooth LE mouse chameleon port.
	PortTypeBLEMouse PortType = "ble_mouse"

	// PortTypeBLEKeyboard is the PortType for a bluetooth LE keyboard chameleon
	// port.
	PortTypeBLEKeyboard PortType = "ble_keyboard"

	// PortTypeBluetoothBase is the PortType for a bluetooth base chameleon port.
	PortTypeBluetoothBase PortType = "bluetooth_base"

	// PortTypeBluetoothTester is the PortType for a bluetooth tester chameleon
	// port.
	PortTypeBluetoothTester PortType = "bluetooth_tester"

	// PortTypeBLEPhone is the PortType for a bluetooth LE phone chameleon port.
	PortTypeBLEPhone PortType = "ble_phone"

	// PortTypeBluetoothAudio is the PortType for a bluetooth audio chameleon
	// port.
	PortTypeBluetoothAudio PortType = "bluetooth_audio"

	// PortTypeBLEFastPair is the PortType for a bluetooth LE fast pair chameleon
	// port.
	PortTypeBLEFastPair PortType = "ble_fast_pair"
)

// String returns this PortType as a string.
func (p PortType) String() string {
	return string(p)
}

// PortID is an id of a chameleon port.
type PortID int

// Int returns this PortID as an int.
func (p PortID) Int() int {
	return int(p)
}

// chameleonV2PortIDToPortTypeMap is a backup map of PortID to PortType for
// chameleon flows that do not correctly support the "GetConnectorType" XMLRPC
// method.
//
// This is based on the static mapping used in chameleon V2, and is not
// accurate for chameleon V3 devices. However, it is expected that all
// chameleon V3 devices should support GetConnectorType for all PortID values so
// this would not be used with a chameleon V3 device.
var chameleonV2PortIDToPortTypeMap = map[PortID]PortType{
	1:  PortTypeDP,
	2:  PortTypeDP,
	3:  PortTypeHDMI,
	4:  PortTypeVGA,
	5:  PortTypeMIC,
	6:  PortTypeAnalogAudioLineIn,
	7:  PortTypeAnalogAudioLineOut,
	8:  PortTypeUSBAudioIn,
	9:  PortTypeUSBAudioOut,
	10: PortTypeUSBKeyboard,
	11: PortTypeUSBTouch,
	12: PortTypeBluetoothHIDKeyboard,
	13: PortTypeBluetoothHIDGamepad,
	14: PortTypeBluetoothHIDMouse,
	15: PortTypeBluetoothHIDCombo,
	16: PortTypeBluetoothHIDJoystick,
	17: PortTypeAVSyncProbe,
	18: PortTypeAudioBoard,
	19: PortTypeMotorBoard,
	20: PortTypeBluetoothHOGKeyboard,
	21: PortTypeBluetoothHOGGamepad,
	22: PortTypeBluetoothHOGMouse,
	23: PortTypeBluetoothHOGCombo,
	24: PortTypeBluetoothHOGJoystick,
	25: PortTypeUSBPrinter,
	26: PortTypeBluetoothA2DPSink,
	27: PortTypeBLEMouse,
	28: PortTypeBLEKeyboard,
	29: PortTypeBluetoothBase,
	30: PortTypeBluetoothTester,
	31: PortTypeBLEPhone,
	32: PortTypeBluetoothAudio,
	33: PortTypeBLEFastPair,
}

// IntsToPortIDs returns an int array as a PortID array.
func IntsToPortIDs(intPortIDs []int) []PortID {
	result := make([]PortID, len(intPortIDs))
	for i, intPortID := range intPortIDs {
		result[i] = PortID(intPortID)
	}
	return result
}

// AudioFileType is a
type AudioFileType string

const (
	// AudioFileTypeRaw denotes the raw audio file type.
	AudioFileTypeRaw AudioFileType = "raw"

	// AudioFileTypeWav denotes the wav audio file type.
	AudioFileTypeWav AudioFileType = "wav"
)

// String returns this AudioFileType as a string.
func (aft AudioFileType) String() string {
	return string(aft)
}

// AudioSampleFormat is the name of an audio sample format. See aplay(1) for
// a list of formats.
type AudioSampleFormat string

const (
	// AudioSampleFormatS8 denotes the S8 audio sample format.
	AudioSampleFormatS8 AudioSampleFormat = "S8"

	// AudioSampleFormatU8 denotes the U8 audio sample format.
	AudioSampleFormatU8 AudioSampleFormat = "U8"

	// AudioSampleFormatS16LE denotes the S16_LE audio sample format.
	AudioSampleFormatS16LE AudioSampleFormat = "S16_LE"

	// AudioSampleFormatS16BE denotes the S16_BE audio sample format.
	AudioSampleFormatS16BE AudioSampleFormat = "S16_BE"

	// AudioSampleFormatU16LE denotes the U16_LE audio sample format.
	AudioSampleFormatU16LE AudioSampleFormat = "U16_LE"

	// AudioSampleFormatU16BE denotes the U16_BE audio sample format.
	AudioSampleFormatU16BE AudioSampleFormat = "U16_BE"

	// AudioSampleFormatS24LE denotes the S24_LE audio sample format.
	AudioSampleFormatS24LE AudioSampleFormat = "S24_LE"

	// AudioSampleFormatS24BE denotes the S24_BE audio sample format.
	AudioSampleFormatS24BE AudioSampleFormat = "S24_BE"

	// AudioSampleFormatU24LE denotes the U24_LE audio sample format.
	AudioSampleFormatU24LE AudioSampleFormat = "U24_LE"

	// AudioSampleFormatU24BE denotes the U24_BE audio sample format.
	AudioSampleFormatU24BE AudioSampleFormat = "U24_BE"

	// AudioSampleFormatS32LE denotes the S32_LE audio sample format.
	AudioSampleFormatS32LE AudioSampleFormat = "S32_LE"

	// AudioSampleFormatS32BE denotes the S32_BE audio sample format.
	AudioSampleFormatS32BE AudioSampleFormat = "S32_BE"

	// AudioSampleFormatU32LE denotes the U32_LE audio sample format.
	AudioSampleFormatU32LE AudioSampleFormat = "U32_LE"

	// AudioSampleFormatU32BE denotes the U32_BE audio sample format.
	AudioSampleFormatU32BE AudioSampleFormat = "U32_BE"

	// AudioSampleFormatFLOATLE denotes the FLOAT_LE audio sample format.
	AudioSampleFormatFLOATLE AudioSampleFormat = "FLOAT_LE"

	// AudioSampleFormatFLOATBE denotes the FLOAT_BE audio sample format.
	AudioSampleFormatFLOATBE AudioSampleFormat = "FLOAT_BE"

	// AudioSampleFormatFLOAT64LE denotes the FLOAT64_LE audio sample format.
	AudioSampleFormatFLOAT64LE AudioSampleFormat = "FLOAT64_LE"

	// AudioSampleFormatFLOAT64BE denotes the FLOAT64_BE audio sample format.
	AudioSampleFormatFLOAT64BE AudioSampleFormat = "FLOAT64_BE"

	// AudioSampleFormatIEC958SubframeLE denotes the IEC958_SUBFRAME_LE audio
	// sample format.
	AudioSampleFormatIEC958SubframeLE AudioSampleFormat = "IEC958_SUBFRAME_LE"

	// AudioSampleFormatIEC958SubframeBE denotes the IEC958_SUBFRAME_BE audio
	// sample format.
	AudioSampleFormatIEC958SubframeBE AudioSampleFormat = "IEC958_SUBFRAME_BE"

	// AudioSampleFormatMULAW denotes the MU_LAW audio sample format.
	AudioSampleFormatMULAW AudioSampleFormat = "MU_LAW"

	// AudioSampleFormatALAW denotes the A_LAW audio sample format.
	AudioSampleFormatALAW AudioSampleFormat = "A_LAW"

	// AudioSampleFormatIMAADPCM denotes the IMA_ADPCM audio sample format.
	AudioSampleFormatIMAADPCM AudioSampleFormat = "IMA_ADPCM"

	// AudioSampleFormatMPEG denotes the MPEG audio sample format.
	AudioSampleFormatMPEG AudioSampleFormat = "MPEG"

	// AudioSampleFormatGSM denotes the GSM audio sample format.
	AudioSampleFormatGSM AudioSampleFormat = "GSM"

	// AudioSampleFormatSpecial denotes the SPECIAL audio sample format.
	AudioSampleFormatSpecial AudioSampleFormat = "SPECIAL"

	// AudioSampleFormatS243LE denotes the S24_3LE audio sample format.
	AudioSampleFormatS243LE AudioSampleFormat = "S24_3LE"

	// AudioSampleFormatS243BE denotes the S24_3BE audio sample format.
	AudioSampleFormatS243BE AudioSampleFormat = "S24_3BE"

	// AudioSampleFormatU243LE denotes the U24_3LE audio sample format.
	AudioSampleFormatU243LE AudioSampleFormat = "U24_3LE"

	// AudioSampleFormatU243BE denotes the U24_3BE audio sample format.
	AudioSampleFormatU243BE AudioSampleFormat = "U24_3BE"

	// AudioSampleFormatS203LE denotes the S20_3LE audio sample format.
	AudioSampleFormatS203LE AudioSampleFormat = "S20_3LE"

	// AudioSampleFormatS203BE denotes the S20_3BE audio sample format.
	AudioSampleFormatS203BE AudioSampleFormat = "S20_3BE"

	// AudioSampleFormatU203LE denotes the U20_3LE audio sample format.
	AudioSampleFormatU203LE AudioSampleFormat = "U20_3LE"

	// AudioSampleFormatU203BE denotes the U20_3BE audio sample format.
	AudioSampleFormatU203BE AudioSampleFormat = "U20_3BE"

	// AudioSampleFormatS183LE denotes the S18_3LE audio sample format.
	AudioSampleFormatS183LE AudioSampleFormat = "S18_3LE"

	// AudioSampleFormatS183BE denotes the S18_3BE audio sample format.
	AudioSampleFormatS183BE AudioSampleFormat = "S18_3BE"

	// AudioSampleFormatU183LE denotes the U18_3LE audio sample format.
	AudioSampleFormatU183LE AudioSampleFormat = "U18_3LE"
)

// String returns this AudioSampleFormat as a string.
func (asf AudioSampleFormat) String() string {
	return string(asf)
}

// AudioBusEndpoint represents an endpoint on audio bus.
// There are four terminals on audio bus. Each terminal has two endpoints of two
// roles, that is, one source and one sink. The role of the
// endpoint is determined from the point of view of audio signal on the audio
// bus. For example, headphone is seen as an output port on Cros device, but
// it is a source endpoint for audio signal on the audio bus.
// Endpoints can be connected to audio bus independently. But in usual cases,
// an audio bus should have no more than one source at a time.
// The following table lists the role of each endpoint.
// Terminal               Endpoint               role
// ---------------------------------------------------------------
// Cros device            Heaphone               source
// Cros device            External Microphone    sink
// Peripheral device      Microphone             source
// Peripheral device      Speaker                sink
// Chameleon FPGA         LineOut                source
// Chameleon FPGA         LineIn                 sink
// Bluetooth module       Output port            source
// Bluetooth module       Input port             sink
//
//	                Peripheral device
//								        o  o
//
// Cros   o <============ Bus 1 =================> o   Chameleon
// device o ============= Bus 2 =================> o   FPGA
//
//								        o  o
//	                Bluetooth module
//
// Each source/sink endpoint has two switches to control the connection
// on audio bus 1 and audio bus 2. So in total there are 16 switches for 8
// endpoints.
type AudioBusEndpoint string

const (
	// AudioBusEndpointCrosHeadphone is the audio bus endpoint name for the
	// CROS headphone.
	AudioBusEndpointCrosHeadphone AudioBusEndpoint = "Cros device headphone"

	// AudioBusEndpointCrosExternalMicrophone is the audio bus endpoint name
	// for the CROS external microphone.
	AudioBusEndpointCrosExternalMicrophone AudioBusEndpoint = "Cros device external microphone"

	// AudioBusEndpointPeripheralMicrophone is the audio bus endpoint name for
	// the peripheral microphone.
	AudioBusEndpointPeripheralMicrophone AudioBusEndpoint = "Peripheral microphone"

	// AudioBusEndpointPeripheralSpeaker is the audio bus endpoint name for
	// the peripheral speaker.
	AudioBusEndpointPeripheralSpeaker AudioBusEndpoint = "Peripheral speaker"

	// AudioBusEndpointFPGALineOut is the audio bus endpoint name for the FPGA
	// line-out.
	AudioBusEndpointFPGALineOut AudioBusEndpoint = "Chameleon FPGA line-out"

	// AudioBusEndpointFPGALineIn is the audio bus endpoint name for the FPGA
	// line-in.
	AudioBusEndpointFPGALineIn AudioBusEndpoint = "Chameleon FPGA line-out"

	// AudioBusEndpointBluetoothOutput is the audio bus endpoint name for
	// bluetooth output.
	AudioBusEndpointBluetoothOutput AudioBusEndpoint = "Bluetooth module output"

	// AudioBusEndpointBluetoothInput is the audio bus endpoint name for
	// bluetooth input.
	AudioBusEndpointBluetoothInput AudioBusEndpoint = "Bluetooth module input"
)

// String returns this AudioBusEndpoint as a string.
func (abe AudioBusEndpoint) String() string {
	return string(abe)
}

// InfoFrameType is a name of an InfoFrame type.
type InfoFrameType string

const (
	// InfoFrameTypeAVI is the avi InfoFrame type.
	InfoFrameTypeAVI InfoFrameType = "avi"

	// InfoFrameTypeAudio is the audio InfoFrame type.
	InfoFrameTypeAudio InfoFrameType = "audio"

	// InfoFrameTypeMPEG is the mpeg InfoFrame type.
	InfoFrameTypeMPEG InfoFrameType = "mpeg"

	// InfoFrameTypeVendor is the vendor InfoFrame type.
	InfoFrameTypeVendor InfoFrameType = "vendor"

	// InfoFrameTypeSPD is the spd InfoFrame type.
	InfoFrameTypeSPD InfoFrameType = "spd"
)

// String returns this InfoFrameType as a string.
func (ift InfoFrameType) String() string {
	return string(ift)
}

// AudioBusNumber represents the audio bus number.
type AudioBusNumber int

// AudioBusNumber values.  Bus 1 and Bus 2 are the supported values.
const (
	AudioBus1 AudioBusNumber = 1
	AudioBus2 AudioBusNumber = 2
)

// Int returns this PortID as an int.
func (abn AudioBusNumber) Int() int {
	return int(abn)
}

// SupportedAudioDataFormat represents the only supported audio data format by chameleon.
var SupportedAudioDataFormat = &AudioDataFormat{
	FileType:     AudioFileTypeRaw,
	SampleFormat: AudioSampleFormatS32LE,
	Channel:      8,
	Rate:         48000,
}
