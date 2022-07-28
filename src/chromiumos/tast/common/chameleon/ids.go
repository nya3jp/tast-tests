// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

// chameleondDefaultXMLRPCPort is the default port for the chameleond XMLRPC
// interface.
const chameleondDefaultXMLRPCPort = 9992

// PortID is an id of a chameleon port.
type PortID int

const (
	// PortDP1 is the PortID for the first DisplayPort chameleon port.
	PortDP1 PortID = 0

	// PortDP2 is the PortID for the second DisplayPort chameleon port.
	PortDP2 PortID = 1

	// PortHDMI is the PortID for the HDMI chameleon port.
	PortHDMI PortID = 2

	// PortVGA is the PortID for the VGA chameleon port.
	PortVGA PortID = 3

	// PortMic is the PortID for the microphone chameleon port.
	PortMic PortID = 4

	// PortAnalogAudioLineIn is the PortID for the audio line-in chameleon port.
	PortAnalogAudioLineIn PortID = 5

	// PortAnalogAudioLineOut is the PortID for the audio line-out chameleon port.
	PortAnalogAudioLineOut PortID = 6

	// PortUSBAudioIn is the PortID for the USB audio-in chameleon port.
	PortUSBAudioIn PortID = 7

	// PortUSBAudioOut is the PortID for the USB audio-out chameleon port.
	PortUSBAudioOut PortID = 8

	// PortUSBKeyboard is the PortID for the keyboard chameleon port.
	PortUSBKeyboard PortID = 9

	// PortUSBTouch is the PortID for the USB touch chameleon port.
	PortUSBTouch PortID = 10

	// PortBluetoothHIDKeyboard is the PortID for the bluetooth HID keyboard
	// chameleon port.
	PortBluetoothHIDKeyboard PortID = 11

	// PortBluetoothHIDGamepad is the PortID for the bluetooth HID gamepad
	// chameleon port.
	PortBluetoothHIDGamepad PortID = 12

	// PortBluetoothHIDMouse is the PortID for the bluetooth HID mouse
	// chameleon port.
	PortBluetoothHIDMouse PortID = 13

	// PortBluetoothHIDCombo is the PortID for the bluetooth HID combo
	// chameleon port.
	PortBluetoothHIDCombo PortID = 14

	// PortBluetoothHIDJoystick is the PortID for the bluetooth HID joystick
	// chameleon port.
	PortBluetoothHIDJoystick PortID = 15

	// PortAVSyncProbe is the PortID for the AV sync probe chameleon port.
	PortAVSyncProbe PortID = 16

	// PortAudioBoard is the PortID for the audio board chameleon port.
	PortAudioBoard PortID = 17

	// PortMotoBoard is the PortID for the motor board chameleon port.
	PortMotoBoard PortID = 18

	// PortBluetoothHOGKeyboard is the PortID for the bluetooth HOG keyboard
	// chameleon port.
	PortBluetoothHOGKeyboard PortID = 19

	// PortBluetoothHOGGamepad is the PortID for the bluetooth HOG gamepad
	// chameleon port.
	PortBluetoothHOGGamepad PortID = 20

	// PortBluetoothHOGMouse is the PortID for the bluetooth HOG mouse
	// chameleon port.
	PortBluetoothHOGMouse PortID = 21

	// PortBluetoothHOGCombo is the PortID for the bluetooth HOG combo
	// chameleon port.
	PortBluetoothHOGCombo PortID = 22

	// PortBluetoothHOGJoystick is the PortID for the bluetooth HOG joystick
	// chameleon port.
	PortBluetoothHOGJoystick PortID = 23

	// PortUSBPrinter is the PortID for the USB printer chameleon port.
	PortUSBPrinter PortID = 24

	// PortBluetoothA2DPSink is the PortID for the bluetooth A2DP sink
	// chameleon port.
	PortBluetoothA2DPSink PortID = 25

	// PortBLEMouse is the PortID for the bluetooth LE mouse chameleon port.
	PortBLEMouse PortID = 26

	// PortBLEKeyboard is the PortID for the bluetooth LE keyboard chameleon
	// port.
	PortBLEKeyboard PortID = 27

	// PortBluetoothBase is the PortID for the bluetooth base chameleon port.
	PortBluetoothBase PortID = 28

	// PortBluetoothTester is the PortID for the bluetooth tester chameleon port.
	PortBluetoothTester PortID = 29

	// PortBLEPhone is the PortID for the bluetooth LE phone chameleon port.
	PortBLEPhone PortID = 30

	// PortBluetoothAudio is the PortID for the bluetooth audio chameleon port.
	PortBluetoothAudio PortID = 31

	// PortBLEFastPair is the PortID for the bluetooth LE fast pair chameleon
	// port.
	PortBLEFastPair PortID = 32
)

// Int returns this PortID as an int.
func (p PortID) Int() int {
	return int(p)
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

// AudioBusEndpoint is a name of an audio bus endpoint.
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
