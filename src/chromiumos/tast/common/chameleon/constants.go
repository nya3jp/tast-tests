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
	PortDP1 PortID = 1

	// PortDP2 is the PortID for the second DisplayPort chameleon port.
	PortDP2 PortID = 2

	// PortHDMI is the PortID for the HDMI chameleon port.
	PortHDMI PortID = 3

	// PortVGA is the PortID for the VGA chameleon port.
	PortVGA PortID = 4

	// PortMic is the PortID for the microphone chameleon port.
	PortMic PortID = 5

	// PortAnalogAudioLineIn is the PortID for the audio line-in chameleon port.
	PortAnalogAudioLineIn PortID = 6

	// PortAnalogAudioLineOut is the PortID for the audio line-out chameleon port.
	PortAnalogAudioLineOut PortID = 7

	// PortUSBAudioIn is the PortID for the USB audio-in chameleon port.
	PortUSBAudioIn PortID = 8

	// PortUSBAudioOut is the PortID for the USB audio-out chameleon port.
	PortUSBAudioOut PortID = 9

	// PortUSBKeyboard is the PortID for the keyboard chameleon port.
	PortUSBKeyboard PortID = 10

	// PortUSBTouch is the PortID for the USB touch chameleon port.
	PortUSBTouch PortID = 11

	// PortBluetoothHIDKeyboard is the PortID for the bluetooth HID keyboard
	// chameleon port.
	PortBluetoothHIDKeyboard PortID = 12

	// PortBluetoothHIDGamepad is the PortID for the bluetooth HID gamepad
	// chameleon port.
	PortBluetoothHIDGamepad PortID = 13

	// PortBluetoothHIDMouse is the PortID for the bluetooth HID mouse
	// chameleon port.
	PortBluetoothHIDMouse PortID = 14

	// PortBluetoothHIDCombo is the PortID for the bluetooth HID combo
	// chameleon port.
	PortBluetoothHIDCombo PortID = 15

	// PortBluetoothHIDJoystick is the PortID for the bluetooth HID joystick
	// chameleon port.
	PortBluetoothHIDJoystick PortID = 16

	// PortAVSyncProbe is the PortID for the AV sync probe chameleon port.
	PortAVSyncProbe PortID = 17

	// PortAudioBoard is the PortID for the audio board chameleon port.
	PortAudioBoard PortID = 18

	// PortMotorBoard is the PortID for the motor board chameleon port.
	PortMotorBoard PortID = 19

	// PortBluetoothHOGKeyboard is the PortID for the bluetooth HOG keyboard
	// chameleon port.
	PortBluetoothHOGKeyboard PortID = 20

	// PortBluetoothHOGGamepad is the PortID for the bluetooth HOG gamepad
	// chameleon port.
	PortBluetoothHOGGamepad PortID = 21

	// PortBluetoothHOGMouse is the PortID for the bluetooth HOG mouse
	// chameleon port.
	PortBluetoothHOGMouse PortID = 22

	// PortBluetoothHOGCombo is the PortID for the bluetooth HOG combo
	// chameleon port.
	PortBluetoothHOGCombo PortID = 23

	// PortBluetoothHOGJoystick is the PortID for the bluetooth HOG joystick
	// chameleon port.
	PortBluetoothHOGJoystick PortID = 24

	// PortUSBPrinter is the PortID for the USB printer chameleon port.
	PortUSBPrinter PortID = 25

	// PortBluetoothA2DPSink is the PortID for the bluetooth A2DP sink
	// chameleon port.
	PortBluetoothA2DPSink PortID = 26

	// PortBLEMouse is the PortID for the bluetooth LE mouse chameleon port.
	PortBLEMouse PortID = 27

	// PortBLEKeyboard is the PortID for the bluetooth LE keyboard chameleon
	// port.
	PortBLEKeyboard PortID = 28

	// PortBluetoothBase is the PortID for the bluetooth base chameleon port.
	PortBluetoothBase PortID = 29

	// PortBluetoothTester is the PortID for the bluetooth tester chameleon port.
	PortBluetoothTester PortID = 30

	// PortBLEPhone is the PortID for the bluetooth LE phone chameleon port.
	PortBLEPhone PortID = 31

	// PortBluetoothAudio is the PortID for the bluetooth audio chameleon port.
	PortBluetoothAudio PortID = 32

	// PortBLEFastPair is the PortID for the bluetooth LE fast pair chameleon
	// port.
	PortBLEFastPair PortID = 33
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
