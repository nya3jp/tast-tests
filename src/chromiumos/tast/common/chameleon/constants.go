// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chameleon is used to communicate with chameleon devices connected to DUTs.
// It communicates with chameleon over XML-RPC.
package chameleon

// PortID represents the id of a port.
type PortID int

// PortId values defined in chameleond
const (
	DP1                  PortID = 1
	DP2                  PortID = 2
	HDMI                 PortID = 3
	VGA                  PortID = 4
	Mic                  PortID = 5
	LineIn               PortID = 6
	LineOut              PortID = 7
	USBAudioIn           PortID = 8
	USBAudioOut          PortID = 9
	USBKeyboard          PortID = 10
	USBTouch             PortID = 11
	BluetoothHIDKeyboard PortID = 12
	BluetoothHIDGamepad  PortID = 13
	BluetoothHIDNouse    PortID = 14
	BluetoothHIDCombo    PortID = 15
	BluetoothHIDJoystick PortID = 16
	AVSyncProbe          PortID = 17
	AudioBoard           PortID = 18
	MotorBoard           PortID = 19
	BluetoothHOGKeyboard PortID = 20
	BluetoothHOGGamepad  PortID = 21
	BluetoothHOGNouse    PortID = 22
	BluetoothHOGCombo    PortID = 23
	BluetoothHOGJoystick PortID = 24
	USBPrinter           PortID = 25
	BluetoothA2DPSink    PortID = 26
	BLEMouse             PortID = 27
	BLEKeyboard          PortID = 28
	BluetoothBase        PortID = 29
	BluetoothTester      PortID = 30
	BLEPhone             PortID = 31
	BluetoothAudio       PortID = 32
	BLEFastPair          PortID = 33
)

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
//	                       Peripheral device
//	                        o  o       o  o
//		o                     bus 1                          o
//
// Cros   o <================================================> o   Chameleon
// device o <================================================> o   FPGA
//
//	o                     bus 2                          o
//	                 o  o       o  o
//	                Bluetooth module
//
// Each source/sink endpoint has two switches to control the connection
// on audio bus 1 and audio bus 2. So in total there are 16 switches for 8
// endpoints.
type AudioBusEndpoint string

// AudioBusEndpoint values defined in chameleond
const (
	CrosHeadphone          AudioBusEndpoint = "Cros device headphone"
	CrosExternalMicrophone AudioBusEndpoint = "Cros device external microphone"
	PeripheralMicrophone   AudioBusEndpoint = "Peripheral microphone"
	PeripheralSpeaker      AudioBusEndpoint = "Peripheral speaker"
	FPGALineIn             AudioBusEndpoint = "Chameleon FPGA line-out"
	FPGALineOut            AudioBusEndpoint = "Chameleon FPGA line-in"
	BluetoothOutput        AudioBusEndpoint = "Bluetooth module output"
	BluetoothInput         AudioBusEndpoint = "Bluetooth module input"
)

// AudioFileType represents the file extension type for the audio file
type AudioFileType string

// Supported Audio file type defined in chameleond
const (
	// .raw extension type
	Raw AudioFileType = "raw"
	// .wav extension type
	Wav AudioFileType = "wav"
)

// AudioSampleFormat represents audio sampling format
type AudioSampleFormat string

// AudioSampleFormat values defined in chameleond
const (
	S8               AudioSampleFormat = "S8"
	U8               AudioSampleFormat = "U8"
	S16LE            AudioSampleFormat = "S16_LE"
	S16BE            AudioSampleFormat = "S16_BE"
	U16LE            AudioSampleFormat = "U16_LE"
	U16BE            AudioSampleFormat = "U16_BE"
	S24LE            AudioSampleFormat = "S24_LE"
	S24BE            AudioSampleFormat = "S24_BE"
	U24LE            AudioSampleFormat = "U24_LE"
	U24BE            AudioSampleFormat = "U24_BE"
	S32LE            AudioSampleFormat = "S32_LE"
	S32BE            AudioSampleFormat = "S32_BE"
	U32LE            AudioSampleFormat = "U32_LE"
	U32BE            AudioSampleFormat = "U32_BE"
	FloatLE          AudioSampleFormat = "FLOAT_LE"
	FLoatBE          AudioSampleFormat = "FLOAT_BE"
	Float64LE        AudioSampleFormat = "FLOAT64_LE"
	Float64BE        AudioSampleFormat = "FLOAT64_BE"
	IEC958SubframeLE AudioSampleFormat = "IEC958_SUBFRAME_LE"
	IEC958SubframeBE AudioSampleFormat = "IEC958_SUBFRAME_BE"
	MULAW            AudioSampleFormat = "MU_LAW"
	ALAW             AudioSampleFormat = "A_LAW"
	IMAADPCM         AudioSampleFormat = "IMA_ADPCM"
	MPEG             AudioSampleFormat = "MPEG"
	GSM              AudioSampleFormat = "GSM"
	Special          AudioSampleFormat = "SPECIAL"
	S243LE           AudioSampleFormat = "S24_3LE"
	S243BE           AudioSampleFormat = "S24_3BE"
	U243LE           AudioSampleFormat = "U24_3LE"
	U243BE           AudioSampleFormat = "U24_3BE"
	S203LE           AudioSampleFormat = "S20_3LE"
	S203BE           AudioSampleFormat = "S20_3BE"
	U203LE           AudioSampleFormat = "U20_3LE"
	U203BE           AudioSampleFormat = "U20_3BE"
	S183LE           AudioSampleFormat = "S18_3LE"
	S183BE           AudioSampleFormat = "S18_3BE"
	U183LE           AudioSampleFormat = "U18_3LE"
)

// SupportdAudioDataFormat represents the only supported audio data format by chameleon
var SupportdAudioDataFormat = map[string]interface{}{
	"file_type":     Raw,
	"sample_format": S32LE,
	"channel":       8,
	"rate":          48000,
}
