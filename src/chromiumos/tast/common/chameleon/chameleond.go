// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"chromiumos/tast/common/chameleon/devices"
	"chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DeviceStatus is a pairing of chameleond device name to its capability status.
type DeviceStatus struct {
	// Device is the name of the chameleon device (e.g. 'HDMI').
	Device string
	// Status is true when the chameleon is capable of using the chameleon device.
	Status bool
}

// AudioDataFormat is a container used to specify common Chameleond method
// parameter and return maps that refer to audio data format attributes.
type AudioDataFormat struct {
	// FileType is the type of audio file.
	FileType AudioFileType
	// SampleFormat is the sample format of the audio file.
	SampleFormat AudioSampleFormat
	// Channel is the number of channels in the audio file.
	Channel int
	// Rate is the sampling rate in Hz of the audio (or zero if unknown).
	Rate int
}

// Map copies the fields of AudioDataFormat to a map, as expected by
// chameleond.
func (adf *AudioDataFormat) Map() map[string]interface{} {
	formatMap := make(map[string]interface{})
	formatMap["file_type"] = adf.FileType.String()
	formatMap["sample_format"] = adf.SampleFormat.String()
	formatMap["channel"] = adf.Channel
	formatMap["rate"] = adf.Rate
	return formatMap
}

// MapToAudioDataFormat converts formatMap to an AudioDataFormat instance.
// The fields of AudioDataFormat are set only if the corresponding entry is
// present in formatMap.
func MapToAudioDataFormat(formatMap map[string]interface{}) (*AudioDataFormat, error) {
	format := &AudioDataFormat{}
	var fileType string
	if err := fetchStringFromGenericMap(formatMap, "file_type", &fileType); err != nil {
		return nil, err
	}
	format.FileType = AudioFileType(fileType)
	var sampleFormat string
	if err := fetchStringFromGenericMap(formatMap, "sample_format", &sampleFormat); err != nil {
		return nil, err
	}
	format.SampleFormat = AudioSampleFormat(sampleFormat)
	if err := fetchIntFromGenericMap(formatMap, "channel", &format.Channel); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(formatMap, "rate", &format.Rate); err != nil {
		return nil, err
	}
	return format, nil
}

func fetchStringFromGenericMap(srcMap map[string]interface{}, key string, dstStr *string) error {
	if genericValue, ok := srcMap[key]; ok {
		strValue, ok := genericValue.(string)
		if !ok {
			return errors.Errorf("failed to parse string at %q from map: %v", key, srcMap)
		}
		*dstStr = strValue
	}
	return nil
}

func fetchIntFromGenericMap(srcMap map[string]interface{}, key string, dstInt *int) error {
	if genericValue, ok := srcMap[key]; ok {
		intValue, ok := genericValue.(int)
		if !ok {
			return errors.Errorf("failed to parse int at %q from map: %v", key, srcMap)
		}
		*dstInt = intValue
	}
	return nil
}

func fetchBoolFromGenericMap(srcMap map[string]interface{}, key string, dstBool *bool) error {
	if genericValue, ok := srcMap[key]; ok {
		boolValue, ok := genericValue.(bool)
		if !ok {
			return errors.Errorf("failed to parse bool at %q from map: %v", key, srcMap)
		}
		*dstBool = boolValue
	}
	return nil
}

func fetchFloat64FromGenericMap(srcMap map[string]interface{}, key string, dstFloat64 *float64) error {
	if genericValue, ok := srcMap[key]; ok {
		float64Value, ok := genericValue.(float64)
		if !ok {
			return errors.Errorf("failed to parse float64 at %q from map: %v", key, srcMap)
		}
		*dstFloat64 = float64Value
	}
	return nil
}

// AudioBoardRoute is a pair of AudioBusEndpoints, with one specified as
// the source endpoint and the other the sink endpoint.
type AudioBoardRoute struct {
	Source AudioBusEndpoint
	Sink   AudioBusEndpoint
}

// ImageView refers to a section of an image.
type ImageView struct {
	// X position of the top-left corner. Optional, defaults to 0.
	X int
	// Y position of the top-left corner. Optional, defaults to 0.
	Y int
	// The Width of the area. Optional, defaults to the full width.
	Width int
	// Height of the area. Optional, defaults to the full width.
	Height int
}

// VideoParams contains relevant video parameters for a video input port.
type VideoParams struct {
	// Clock refers to the "clock" param.
	Clock float64
	// HTotal refers to the "htotal" param.
	HTotal int
	// HActive refers to the "hactive" param.
	HActive int
	// HSyncWidth refers to the "hsync_width" param.
	HSyncWidth int
	// HSyncOffset refers to the "hsync_offset" param.
	HSyncOffset int
	// HSyncPolarity refers to the "hsync_polarity" param.
	HSyncPolarity int
	// VTotal refers to the "vtotal" param.
	VTotal int
	// VActive refers to the "vactive" param.
	VActive int
	// VSyncWidth refers to the "vsync_width" param.
	VSyncWidth int
	// VSyncOffset refers to the "vsync_offset" param.
	VSyncOffset int
	// VSyncPolarity refers to the "vsync_polarity" param.
	VSyncPolarity int
	// BPC refers to the "bpc" param.
	BPC int
	// Interlaced refers to the "interlaced" param.
	Interlaced bool
}

// MapToVideoParams converts paramsMap to a VideoParams instance.
// The fields of VideoParams are set only if the corresponding entry is
// present in paramsMap.
func MapToVideoParams(paramsMap map[string]interface{}) (*VideoParams, error) {
	params := &VideoParams{}
	if err := fetchFloat64FromGenericMap(paramsMap, "clock", &params.Clock); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "htotal", &params.HTotal); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "hactive", &params.HActive); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "hsync_width", &params.HSyncWidth); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "hsync_offset", &params.HSyncOffset); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "hsync_polarity", &params.HSyncPolarity); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "vtotal", &params.VTotal); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "vactive", &params.VActive); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "vsync_width", &params.VSyncWidth); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "vsync_offset", &params.VSyncOffset); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "vsync_polarity", &params.VSyncPolarity); err != nil {
		return nil, err
	}
	if err := fetchIntFromGenericMap(paramsMap, "bpc", &params.BPC); err != nil {
		return nil, err
	}
	if err := fetchBoolFromGenericMap(paramsMap, "interlaced", &params.Interlaced); err != nil {
		return nil, err
	}
	return params, nil
}

// Chameleond is an interface for making RPC calls to a chameleond daemon.
//
// This is based off of the Python class "chameleond.interface.ChameleondInterface"
// from the chameleon source. Refer to that source for more complete
// documentation.
type Chameleond interface {
	xmlrpc.RPCInterface

	// Reset calls the Chameleond RPC method of the same name.
	// Resets Chameleon board.
	Reset(ctx context.Context) error

	// GetDetectedStatus calls the Chameleond RPC method of the same name.
	// Returns the detected status of all devices. This can be used to determine
	// the capability of the Chameleon board.
	GetDetectedStatus(ctx context.Context) ([]DeviceStatus, error)

	// HasDevice calls the Chameleond RPC method of the same name.
	// Returns True if there is a device.
	HasDevice(ctx context.Context, deviceID PortID) (bool, error)

	// GetSupportedPorts calls the Chameleond RPC method of the same name.
	// Returns all supported ports on the board.
	GetSupportedPorts(ctx context.Context) ([]PortID, error)

	// GetSupportedInputs calls the Chameleond RPC method of the same name.
	// Returns all supported input ports on the board.
	GetSupportedInputs(ctx context.Context) ([]PortID, error)

	// GetSupportedOutputs calls the Chameleond RPC method of the same name.
	// Returns all supported output ports on the board.
	GetSupportedOutputs(ctx context.Context) ([]PortID, error)

	// IsPhysicalPlugged calls the Chameleond RPC method of the same name.
	// Returns true if the physical cable is plugged between DUT and Chameleon.
	IsPhysicalPlugged(ctx context.Context, portID PortID) (bool, error)

	// ProbePorts calls the Chameleond RPC method of the same name.
	// Probes all the connected ports on Chameleon board.
	ProbePorts(ctx context.Context) (portsConnectedToDut []PortID, err error)

	// ProbeInputs calls the Chameleond RPC method of the same name.
	// Probes all the connected input ports on Chameleon board.
	ProbeInputs(ctx context.Context) (inputPortsConnectedToDut []PortID, err error)

	// ProbeOutputs calls the Chameleond RPC method of the same name.
	// Probes all the connected output ports on Chameleon board.
	ProbeOutputs(ctx context.Context) (outputPortsConnectedToDut []PortID, err error)

	// GetConnectorType calls the Chameleond RPC method of the same name.
	// Returns the connector type string as a PortType.
	GetConnectorType(ctx context.Context, portID PortID) (PortType, error)

	// IsPlugged calls the Chameleond RPC method of the same name.
	// Returns true if the port is emulated as plugged.
	IsPlugged(ctx context.Context, portID PortID) (bool, error)

	// Plug calls the Chameleond RPC method of the same name.
	// Emulates plug, like asserting HPD line to high on a video port.
	Plug(ctx context.Context, portID PortID) error

	// Unplug calls the Chameleond RPC method of the same name.
	// Emulates unplug, like de-asserting HPD line to low on a video port.
	Unplug(ctx context.Context, portID PortID) error

	// GetMacAddress calls the Chameleond RPC method of the same name.
	// Gets the MAC address of this Chameleon.
	GetMacAddress(ctx context.Context) (string, error)

	// HasAudioSupport calls the Chameleond RPC method of the same name.
	// Returns true if the port has audio support.
	HasAudioSupport(ctx context.Context, portID PortID) (bool, error)

	// GetAudioChannelMapping calls the Chameleond RPC method of the same name.
	// Obtains the channel mapping for an audio port.
	//
	// Audio channels are not guaranteed to not be swapped. Clients can use the
	// channel mapping to match a wire channel to a Chameleon channel.
	//
	// This method may only be called when audio capture or playback is in
	// progress.
	//
	// Returns an array of integers. There is one element per Chameleon channel.
	// For audio input ports, each element indicates which input channel the
	// capture channel is mapped to. For audio output ports, each element
	// indicates which output channel the playback channel is mapped to. As a
	// special case, -1 means the channel isn't mapped.
	GetAudioChannelMapping(ctx context.Context, portID PortID) ([]int, error)

	// GetAudioFormat calls the Chameleond RPC method of the same name.
	// Gets the format currently used by audio capture.
	GetAudioFormat(ctx context.Context, portID PortID) (*AudioDataFormat, error)

	// StartCapturingAudio calls the Chameleond RPC method of the same name.
	// Starts capturing audio.
	StartCapturingAudio(ctx context.Context, portID PortID, hasFile bool) error

	// StopCapturingAudio calls the Chameleond RPC method of the same name.
	// Stops capturing audio and returns recorded data path and format.
	StopCapturingAudio(ctx context.Context, portID PortID) (path string, format AudioSampleFormat, err error)

	// StartPlayingAudio calls the Chameleond RPC method of the same name.
	// Starts playing audio data at given path using given format and port.
	//
	// Note that SupportedAudioDataFormat is the only format that chameleons are
	// expected to support.
	StartPlayingAudio(ctx context.Context, portID PortID, path string, format *AudioDataFormat) error

	// StartPlayingEcho calls the Chameleond RPC method of the same name.
	// Echoes audio data received from inputID and plays to portID.
	StartPlayingEcho(ctx context.Context, portID, inputID PortID) error

	// StopPlayingAudio calls the Chameleond RPC method of the same name.
	// Stops playing audio from port_id port.
	StopPlayingAudio(ctx context.Context, portID PortID) error

	// AudioBoardConnect calls the Chameleond RPC method of the same name.
	// Connects an endpoint to an audio bus.
	AudioBoardConnect(ctx context.Context, busNumber AudioBusNumber, endpoint AudioBusEndpoint) error

	// AudioBoardDisconnect calls the Chameleond RPC method of the same name.
	// Disconnects an endpoint to an audio bus.
	AudioBoardDisconnect(ctx context.Context, busNumber AudioBusNumber, endpoint AudioBusEndpoint) error

	// AudioBoardGetRoutes calls the Chameleond RPC method of the same name.
	// Gets a list of routes on audio bus.
	AudioBoardGetRoutes(ctx context.Context, busNumber AudioBusNumber) ([]AudioBoardRoute, error)

	// AudioBoardClearRoutes calls the Chameleond RPC method of the same name.
	// Clears the routes on an audio bus.
	AudioBoardClearRoutes(ctx context.Context, busNumber AudioBusNumber) error

	// AudioBoardHasJackPlugger calls the Chameleond RPC method of the same name.
	// Returns true if there is jack plugger on audio board.
	// An audio board must have the motor cable connected in order to control
	// jack plugger of audio box.
	AudioBoardHasJackPlugger(ctx context.Context) (bool, error)

	// AudioBoardAudioJackPlug calls the Chameleond RPC method of the same name.
	// Plugs audio jack to connect audio board and Cros device.
	AudioBoardAudioJackPlug(ctx context.Context) error

	// AudioBoardAudioJackUnplug calls the Chameleond RPC method of the same name.
	// Unplugs audio jack to disconnect audio board and Cros device.
	AudioBoardAudioJackUnplug(ctx context.Context) error

	// SetUSBDriverPlaybackConfigs calls the Chameleond RPC method of the same name.
	// Updates the corresponding playback configurations.
	SetUSBDriverPlaybackConfigs(ctx context.Context, playbackDataFormat *AudioDataFormat) error

	// SetUSBDriverCaptureConfigs calls the Chameleond RPC method of the same name.
	// Updates the corresponding capture configurations.
	SetUSBDriverCaptureConfigs(ctx context.Context, captureDataFormat *AudioDataFormat) error

	// AudioBoardResetBluetooth calls the Chameleond RPC method of the same name.
	// Resets bluetooth module on audio board.
	AudioBoardResetBluetooth(ctx context.Context) error

	// AudioBoardDisableBluetooth calls the Chameleond RPC method of the same name.
	// Disables bluetooth module on audio board.
	AudioBoardDisableBluetooth(ctx context.Context) error

	// AudioBoardIsBluetoothEnabled calls the Chameleond RPC method of the same name.
	// Returns true if the bluetooth module on audio board is enabled.
	AudioBoardIsBluetoothEnabled(ctx context.Context) (bool, error)

	// ResetBluetoothRef calls the Chameleond RPC method of the same name.
	// Resets BTREF.
	ResetBluetoothRef(ctx context.Context) error

	// DisableBluetoothRef calls the Chameleond RPC method of the same name.
	// Disables BTREF.
	DisableBluetoothRef(ctx context.Context) error

	// IsBluetoothRefDisabled calls the Chameleond RPC method of the same name.
	// Returns true if BTREF is enabled.
	IsBluetoothRefDisabled(ctx context.Context) (bool, error)

	// TriggerLinkFailure calls the Chameleond RPC method of the same name.
	// Triggers a link failure on the port.
	TriggerLinkFailure(ctx context.Context, portID PortID) error

	// HasVideoSupport calls the Chameleond RPC method of the same name.
	// Returns true if the port has audio support.
	HasVideoSupport(ctx context.Context, portID PortID) (bool, error)

	// SetVgaMode calls the Chameleond RPC method of the same name.
	// Sets the mode for VGA monitor.
	SetVgaMode(ctx context.Context, portID PortID, mode string) error

	// WaitVideoInputStable calls the Chameleond RPC method of the same name.
	// Waits until the video input is stable or the timeout is reached.
	WaitVideoInputStable(ctx context.Context, portID PortID, timeoutSeconds float64) (stableBeforeTimeout bool, err error)

	// CreateEdid calls the Chameleond RPC method of the same name.
	// Creates an internal record of EDID using the given byte array.
	CreateEdid(ctx context.Context, edid []byte) (edidID int, err error)

	// DestroyEdid calls the Chameleond RPC method of the same name.
	// Destroys the internal record of EDID. The internal data will be freed.
	DestroyEdid(ctx context.Context, edidID int) error

	// SetDdcState calls the Chameleond RPC method of the same name.
	// Sets the enabled/disabled state of DDC bus on the given video input.
	SetDdcState(ctx context.Context, portID PortID, enabled bool) error

	// IsDdcEnabled calls the Chameleond RPC method of the same name.
	// Returns true if the DDC bus is enabled on the given video input.
	IsDdcEnabled(ctx context.Context, portID PortID) (bool, error)

	// ReadEdid calls the Chameleond RPC method of the same name.
	// Reads the EDID content of the selected video input on Chameleon.
	ReadEdid(ctx context.Context, portID PortID) (edid []byte, err error)

	// ApplyEdid calls the Chameleond RPC method of the same name.
	// Applies the EDID to the selected video input.
	//
	// Note that this method doesn't pulse the HPD line. Should call Plug(),
	// Unplug(), or FireHpdPulse() later.
	ApplyEdid(ctx context.Context, portID PortID, edidID int) error

	// UnplugHPD calls the Chameleond RPC method of the same name.
	// Only de-assert HPD line to low on a video port.
	UnplugHPD(ctx context.Context, portID PortID) error

	// FireHpdPulse calls the Chameleond RPC method of the same name.
	// Fires one or more HPD pulse (low -> high -> low -> ...).
	FireHpdPulse(ctx context.Context, portID PortID, deAssertIntervalMicroSec, assertIntervalMicroSec, repeatCount, endLevel int) error

	// FireMixedHpdPulses calls the Chameleond RPC method of the same name.
	// Fires one or more HPD pulses, starting at low, of mixed widths.
	//
	// One must specify a list of segment widths in the widthsMsec argument where
	// widthsMsec[0] is the width of the first low segment, widthsMsec[1] is that
	// of the first high segment, widthsMsec[2] is that of the second low segment,
	// etc.
	// The HPD line stops at low if even number of segment widths are specified;
	// otherwise, it stops at high.
	//
	// The method is equivalent to a series of calls to Unplug() and Plug()
	// separated by specified pulse widths.
	FireMixedHpdPulses(ctx context.Context, portID PortID, widthsMsec []int) error

	// ScheduleHpdToggle calls the Chameleond RPC method of the same name.
	// Schedules one HPD Toggle, with a delay between the toggle.
	ScheduleHpdToggle(ctx context.Context, portID PortID, delayMs int, risingEdge bool) error

	// SetContentProtection calls the Chameleond RPC method of the same name.
	// Sets the content protection state on the port.
	SetContentProtection(ctx context.Context, portID PortID, enabled bool) error

	// IsContentProtectionEnabled calls the Chameleond RPC method of the same name.
	// Returns True if the content protection is enabled on the port.
	IsContentProtectionEnabled(ctx context.Context, portID PortID) (bool, error)

	// IsVideoInputEncrypted calls the Chameleond RPC method of the same name.
	// Returns True if the video input on the port is encrypted.
	IsVideoInputEncrypted(ctx context.Context, portID PortID) (bool, error)

	// DumpPixels calls the Chameleond RPC method of the same name.
	// Dumps the raw pixel array of the selected area.
	// If view is nil, the whole screen is captured.
	DumpPixels(ctx context.Context, portID PortID, view *ImageView) error

	// GetMaxFrameLimit calls the Chameleond RPC method of the same name.
	// Gets the maximal number of frames which are accommodated in the buffer.
	//
	// The limit depends on the size of the internal buffer on the board and the
	// size of area to capture (full screen or cropped area).
	GetMaxFrameLimit(ctx context.Context, portID PortID, width, height int) (int, error)

	// StartCapturingVideo calls the Chameleond RPC method of the same name.
	// Starts video capturing continuously on the given video input.
	// If view is nil, the whole screen is captured.
	StartCapturingVideo(ctx context.Context, portID PortID, view *ImageView) error

	// StopCapturingVideo calls the Chameleond RPC method of the same name.
	// Stops video capturing which was started previously.
	//
	// Waits until the captured frame count reaches stopIndex. If not given, it
	// stops immediately. Note that the captured frame of stop_index should not be
	// read.
	StopCapturingVideo(ctx context.Context, stopIndex int) error

	// CaptureVideo calls the Chameleond RPC method of the same name.
	// Captures the video stream on the given video input to the buffer.
	// If view is nil, the whole screen is captured.
	CaptureVideo(ctx context.Context, portID PortID, totalFrame int, view *ImageView) error

	// GetCapturedFrameCount calls the Chameleond RPC method of the same name.
	// Gets the total count of the captured frames.
	GetCapturedFrameCount(ctx context.Context) (int, error)

	// GetCapturedResolution calls the Chameleond RPC method of the same name.
	// Gets the resolution of the captured frame.
	// If a cropping area is specified on capturing, the cropped resolution is
	// returned.
	GetCapturedResolution(ctx context.Context) (width, height int, err error)

	// ReadCapturedFrame calls the Chameleond RPC method of the same name.
	// Reads the content of the captured frame from the buffer.
	ReadCapturedFrame(ctx context.Context, frameIndex int) (pixels []byte, err error)

	// CacheFrameThumbnail calls the Chameleond RPC method of the same name.
	// Caches the thumbnail of the dumped field to a temp file.
	CacheFrameThumbnail(ctx context.Context, frameIndex, ratio int) (thumbnailID int, err error)

	// GetCapturedChecksums calls the Chameleond RPC method of the same name.
	// Gets the list of checksums of the captured frames.
	GetCapturedChecksums(ctx context.Context, startIndex, stopIndex int) (checksums []uint32, err error)

	// GetCapturedHistograms calls the Chameleond RPC method of the same name.
	// Gets the list of histograms of the captured frames.
	GetCapturedHistograms(ctx context.Context, startIndex, stopIndex int) (histograms [][]float64, err error)

	// ComputePixelChecksum calls the Chameleond RPC method of the same name.
	// Computes the checksum of pixels in the selected area.
	// If view is nil, the whole screen is computed.
	ComputePixelChecksum(ctx context.Context, portID PortID, view *ImageView) (checksum uint32, err error)

	// DetectResolution calls the Chameleond RPC method of the same name.
	// Detects the video source resolution.
	DetectResolution(ctx context.Context, portID PortID) (width, height int, err error)

	// GetVideoParams calls the Chameleond RPC method of the same name.
	// Gets video parameters.
	GetVideoParams(ctx context.Context, portID PortID) (*VideoParams, error)

	// GetLastInfoFrame calls the Chameleond RPC method of the same name.
	// Obtains the last received InfoFrame of the specified type.
	GetLastInfoFrame(ctx context.Context, portID PortID, infoFrameType InfoFrameType) (version int, payload []byte, length int, err error)

	// AudioBoardDevice returns an RPC interface for making RPC calls to the
	// chameleon audio board device.
	AudioBoardDevice() devices.ChameleonDevice

	// AVSyncProbeDevice returns an RPC interface for making RPC calls to the
	// chameleon AV sync probe device.
	AVSyncProbeDevice() devices.ChameleonDevice

	// BLEFastPair returns an RPC interface for making RPC calls to the
	// chameleon bluetooth LE fast pair device.
	BLEFastPair() bluetooth.FastPairPeripheral

	// BLEKeyboard returns an RPC interface for making RPC calls to the
	// chameleon bluetooth LE keyboard device.
	BLEKeyboard() bluetooth.KeyboardPeripheral

	// BLEMouse returns an RPC interface for making RPC calls to the
	// chameleon bluetooth LE mouse device.
	BLEMouse() bluetooth.MousePeripheral

	// BLEPhone returns an RPC interface for making RPC calls to the
	// chameleon bluetooth LE phone device.
	BLEPhone() bluetooth.PhonePeripheral

	// BluetoothA2DPSink returns an RPC interface for making RPC calls to the
	// chameleon bluetooth A2DP sink device.
	BluetoothA2DPSink() devices.ChameleonDevice

	// BluetoothAudioDevice returns an RPC interface for making RPC calls to
	// the chameleon bluetooth audio device.
	BluetoothAudioDevice() bluetooth.AudioPeripheral

	// BluetoothBaseDevice returns an RPC interface for making RPC calls to the
	// chameleon bluetooth base device.
	BluetoothBaseDevice() bluetooth.BasePeripheral

	// BluetoothKeyboardDevice returns an RPC interface for making RPC calls to
	// the chameleon bluetooth keyboard device.
	BluetoothKeyboardDevice() bluetooth.KeyboardPeripheral

	// BluetoothMouseDevice returns an RPC interface for making RPC calls to
	// the chameleon bluetooth mouse device.
	BluetoothMouseDevice() bluetooth.MousePeripheral

	// BluetoothTesterDevice returns an RPC interface for making RPC calls to
	// the chameleon bluetooth tester device.
	BluetoothTesterDevice() devices.ChameleonDevice

	// MotorBoardDevice returns an RPC interface for making RPC calls to the
	// chameleon motor board device.
	MotorBoardDevice() devices.ChameleonDevice

	// PrinterDevice returns an RPC interface for making RPC calls to the
	// chameleon printer device.
	PrinterDevice() devices.ChameleonDevice

	// FetchSupportedPortIDsByType returns all supported port ids (PortID) mapped
	// by their connector type (PortType).
	//
	// This is not a direct Chameleond RPC method call, but does make calls to
	// RPC methods to obtain this information.
	//
	// Supported PortID values are identified by the result of calling
	// GetSupportedPorts and each PortType is identified by calling
	// GetConnectorType on each PortID. Multiple PortID values may have the same
	// PortType, so the values of the map are sorted PortID arrays.
	FetchSupportedPortIDsByType(ctx context.Context) (map[PortType][]PortID, error)

	// FetchSupportedPortTypes returns all the PortType values that this device
	// supports.
	//
	// This is not a direct Chameleond RPC method call, but does make calls to
	// RPC methods to obtain this information.
	FetchSupportedPortTypes(ctx context.Context) ([]PortType, error)

	// FetchSupportedPortIDByType returns the device-specific integer ID of the
	// port with the given PortType and index. The PortType matching is
	// case-insensitive.
	//
	// This is not a direct Chameleond RPC method call, but does make calls to
	// RPC methods to obtain this information.
	FetchSupportedPortIDByType(ctx context.Context, portType PortType, index int) (PortID, error)
}

// NewChameleond creates a new Chameleond object for communicating with a
// chameleond instance.
//
// connSpec holds chameleond's location, either as "host:port" or just "host"
// (to use the default port).
func NewChameleond(ctx context.Context, connSpec string) (Chameleond, error) {
	testing.ContextLogf(ctx, "New chameleon - conSpec: %s", connSpec)
	host, port, err := parseConnSpec(connSpec)
	if err != nil {
		return nil, err
	}
	ch := NewCommonChameleond(xmlrpc.New(host, port))
	// Validate XMLRPC client by making a simple method call.
	_, err = ch.GetSupportedPorts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to communicate with Chameleond device with a test call to GetSupportedPorts")
	}
	return ch, nil
}

// parseConnSpec parses a connection host:port string and returns the
// components.
func parseConnSpec(c string) (host string, port int, err error) {
	parts := strings.Split(c, ":")
	if len(parts[0]) == 0 {
		// If no host, return error.
		return "", 0, errors.Errorf("no host provided in spec %q", c)
	}
	if len(parts) == 1 {
		// If no port, return default one.
		return parts[0], chameleondDefaultXMLRPCPort, nil
	}
	if len(parts) == 2 {
		port, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, errors.Errorf("got invalid port int in spec %q", c)
		}
		return parts[0], port, nil
	}

	return "", 0, errors.Errorf("got invalid connection spec %q", c)
}

// CommonChameleond is a base implementation of Chameleond that provides methods
// for making XMLRPC calls to a chameleond daemon.
// See the Chameleond interface for more detailed documentation.
type CommonChameleond struct {
	xmlrpc.CommonRPCInterface
	audioBoardDevice        devices.ChameleonDevice
	aVSyncProbeDevice       devices.ChameleonDevice
	bLEFastPair             bluetooth.FastPairPeripheral
	bLEKeyboard             bluetooth.KeyboardPeripheral
	bLEMouse                bluetooth.MousePeripheral
	bLEPhone                bluetooth.PhonePeripheral
	bluetoothA2DPSink       devices.ChameleonDevice
	bluetoothAudioDevice    bluetooth.AudioPeripheral
	bluetoothBaseDevice     bluetooth.BasePeripheral
	bluetoothKeyboardDevice bluetooth.KeyboardPeripheral
	bluetoothMouseDevice    bluetooth.MousePeripheral
	bluetoothTesterDevice   devices.ChameleonDevice
	motorBoardDevice        devices.ChameleonDevice
	printerDevice           devices.ChameleonDevice
	supportedPortIDsByType  map[PortType][]PortID
}

// NewCommonChameleond creates a new instance of CommonChameleond with all
// device interfaces initialized with their appropriate XMLRPC method suffixes.
func NewCommonChameleond(xmlrpcClient *xmlrpc.XMLRpc) *CommonChameleond {
	return &CommonChameleond{
		CommonRPCInterface:      *xmlrpc.NewCommonRPCInterface(xmlrpcClient, ""),
		audioBoardDevice:        devices.NewCommonChameleonDevice(xmlrpcClient, "audio_board."),
		aVSyncProbeDevice:       devices.NewCommonChameleonDevice(xmlrpcClient, "avsync_probe."),
		bLEFastPair:             bluetooth.NewCommonFastPairPeripheral(xmlrpcClient, "ble_fast_pair."),
		bLEKeyboard:             bluetooth.NewCommonKeyboardPeripheral(xmlrpcClient, "ble_keyboard."),
		bLEMouse:                bluetooth.NewCommonMousePeripheral(xmlrpcClient, "ble_mouse."),
		bLEPhone:                bluetooth.NewCommonPhonePeripheral(xmlrpcClient, "ble_phone."),
		bluetoothA2DPSink:       devices.NewCommonChameleonDevice(xmlrpcClient, "bluetooth_a2dp_sink."),
		bluetoothAudioDevice:    bluetooth.NewCommonAudioPeripheral(xmlrpcClient, "bluetooth_audio."),
		bluetoothBaseDevice:     bluetooth.NewCommonBasePeripheral(xmlrpcClient, "bluetooth_base."),
		bluetoothKeyboardDevice: bluetooth.NewCommonKeyboardPeripheral(xmlrpcClient, "bluetooth_keyboard."),
		bluetoothMouseDevice:    bluetooth.NewCommonMousePeripheral(xmlrpcClient, "bluetooth_mouse."),
		bluetoothTesterDevice:   devices.NewCommonChameleonDevice(xmlrpcClient, "bluetooth_tester."),
		motorBoardDevice:        devices.NewCommonChameleonDevice(xmlrpcClient, "motor_board."),
		printerDevice:           devices.NewCommonChameleonDevice(xmlrpcClient, "printer."),
		supportedPortIDsByType:  nil,
	}
}

func (c *CommonChameleond) callForPortIDs(ctx context.Context, callBuilder *xmlrpc.CallBuilder) ([]PortID, error) {
	intPortIDs, err := callBuilder.CallForInts(ctx)
	if err != nil {
		return nil, err
	}
	return IntsToPortIDs(intPortIDs), nil
}

// Reset calls the Chameleond RPC method of the same name.
// This implements Chameleond.Reset, see that for more details.
func (c *CommonChameleond) Reset(ctx context.Context) error {
	return c.RPC("Reset").Call(ctx)
}

// GetDetectedStatus calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetDetectedStatus, see that for more details.
func (c *CommonChameleond) GetDetectedStatus(ctx context.Context) ([]DeviceStatus, error) {
	var rawResult [][]interface{}
	err := c.RPC("GetDetectedStatus").Returns(&rawResult).Call(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make([]DeviceStatus, len(rawResult))
	for i, statusTuple := range rawResult {
		if len(rawResult) != 2 {
			return nil, errors.Errorf("failed to unmarshall GetDetectedStatus return, raw return value: %v", rawResult)
		}
		deviceStatus := DeviceStatus{}
		device, ok := statusTuple[0].(string)
		if !ok {
			return nil, errors.Errorf("failed to unmarshall GetDetectedStatus return, raw return value: %v", rawResult)
		}
		deviceStatus.Device = device
		status, ok := statusTuple[1].(bool)
		if !ok {
			return nil, errors.Errorf("failed to unmarshall GetDetectedStatus return, raw return value: %v", rawResult)
		}
		deviceStatus.Status = status
		statuses[i] = deviceStatus
	}
	return statuses, nil
}

// HasDevice calls the Chameleond RPC method of the same name.
// This implements Chameleond.HasDevice, see that for more details.
func (c *CommonChameleond) HasDevice(ctx context.Context, deviceID PortID) (bool, error) {
	return c.RPC("HasDevice").Args(deviceID.Int()).CallForBool(ctx)
}

// GetSupportedPorts calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetSupportedPorts, see that for more details.
func (c *CommonChameleond) GetSupportedPorts(ctx context.Context) ([]PortID, error) {
	return c.callForPortIDs(ctx, c.RPC("GetSupportedPorts"))
}

// GetSupportedInputs calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetSupportedInputs, see that for more details.
func (c *CommonChameleond) GetSupportedInputs(ctx context.Context) ([]PortID, error) {
	return c.callForPortIDs(ctx, c.RPC("GetSupportedInputs"))
}

// GetSupportedOutputs calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetSupportedOutputs, see that for more details.
func (c *CommonChameleond) GetSupportedOutputs(ctx context.Context) ([]PortID, error) {
	return c.callForPortIDs(ctx, c.RPC("GetSupportedOutputs"))
}

// IsPhysicalPlugged calls the Chameleond RPC method of the same name.
// This implements Chameleond.IsPhysicalPlugged, see that for more details.
func (c *CommonChameleond) IsPhysicalPlugged(ctx context.Context, portID PortID) (bool, error) {
	return c.RPC("IsPhysicalPlugged").Args(portID.Int()).CallForBool(ctx)
}

// ProbePorts calls the Chameleond RPC method of the same name.
// This implements Chameleond.ProbePorts, see that for more details.
func (c *CommonChameleond) ProbePorts(ctx context.Context) (portsConnectedToDut []PortID, err error) {
	return c.callForPortIDs(ctx, c.RPC("ProbePorts"))
}

// ProbeInputs calls the Chameleond RPC method of the same name.
// This implements Chameleond.ProbeInputs, see that for more details.
func (c *CommonChameleond) ProbeInputs(ctx context.Context) (inputPortsConnectedToDut []PortID, err error) {
	return c.callForPortIDs(ctx, c.RPC("ProbeInputs"))
}

// ProbeOutputs calls the Chameleond RPC method of the same name.
// This implements Chameleond.ProbeOutputs, see that for more details.
func (c *CommonChameleond) ProbeOutputs(ctx context.Context) (outputPortsConnectedToDut []PortID, err error) {
	return c.callForPortIDs(ctx, c.RPC("ProbeOutputs"))
}

// GetConnectorType calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetConnectorType, see that for more details.
//
// Since some device flows, notably those for bluetooth ports, fail to return
// a value for this RPC method, if the RPC call fails a backup PortID to
// PortType map based on chameleon V2 values is used to provide a usable
// PortType. While this is technically not a PortType known to the chameleon
// device, it is one that can be used in FetchSupportedPortIDByType to get
// the corresponding PortID later for use in RPC methods.
func (c *CommonChameleond) GetConnectorType(ctx context.Context, portID PortID) (PortType, error) {
	portTypeStr, err := c.RPC("GetConnectorType").Args(portID.Int()).CallForString(ctx)
	if err != nil {
		if portType, ok := chameleonV2PortIDToPortTypeMap[portID]; ok {
			testing.ContextLogf(ctx, "Chameleond GetConnectorType call to device %q failed. Used legacy Chameleond V2 ID map to identify port %d as %q for client-side port id lookup",
				c.Host(), portID, portType)
			return portType, nil
		}
		return "", err
	}
	return PortType(portTypeStr), nil
}

// IsPlugged calls the Chameleond RPC method of the same name.
// This implements Chameleond.IsPlugged, see that for more details.
func (c *CommonChameleond) IsPlugged(ctx context.Context, portID PortID) (bool, error) {
	return c.RPC("IsPlugged").Args(portID.Int()).CallForBool(ctx)
}

// Plug calls the Chameleond RPC method of the same name.
// This implements Chameleond.Plug, see that for more details.
func (c *CommonChameleond) Plug(ctx context.Context, portID PortID) error {
	return c.RPC("Plug").Args(portID.Int()).Call(ctx)
}

// Unplug calls the Chameleond RPC method of the same name.
// This implements Chameleond.Unplug, see that for more details.
func (c *CommonChameleond) Unplug(ctx context.Context, portID PortID) error {
	return c.RPC("Unplug").Args(portID.Int()).Call(ctx)
}

// GetMacAddress calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetMacAddress, see that for more details.
func (c *CommonChameleond) GetMacAddress(ctx context.Context) (string, error) {
	return c.RPC("GetMacAddress").CallForString(ctx)
}

// HasAudioSupport calls the Chameleond RPC method of the same name.
// This implements Chameleond.HasAudioSupport, see that for more details.
func (c *CommonChameleond) HasAudioSupport(ctx context.Context, portID PortID) (bool, error) {
	return c.RPC("HasAudioSupport").Args(portID.Int()).CallForBool(ctx)
}

// GetAudioChannelMapping calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetAudioChannelMapping, see that for more details.
func (c *CommonChameleond) GetAudioChannelMapping(ctx context.Context, portID PortID) ([]int, error) {
	return c.RPC("GetAudioChannelMapping").Args(portID.Int()).CallForInts(ctx)
}

// GetAudioFormat calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetAudioFormat, see that for more details.
func (c *CommonChameleond) GetAudioFormat(ctx context.Context, portID PortID) (*AudioDataFormat, error) {
	formatMap := make(map[string]interface{})
	err := c.RPC("GetAudioFormat").Args(portID.Int()).Returns(&formatMap).Call(ctx)
	if err != nil {
		return nil, err
	}
	return MapToAudioDataFormat(formatMap)
}

// StartCapturingAudio calls the Chameleond RPC method of the same name.
// This implements Chameleond.StartCapturingAudio, see that for more details.
func (c *CommonChameleond) StartCapturingAudio(ctx context.Context, portID PortID, hasFile bool) error {
	return c.RPC("StartCapturingAudio").Args(portID.Int(), hasFile).Call(ctx)
}

// StopCapturingAudio calls the Chameleond RPC method of the same name.
// This implements Chameleond.StopCapturingAudio, see that for more details.
func (c *CommonChameleond) StopCapturingAudio(ctx context.Context, portID PortID) (path string, format AudioSampleFormat, err error) {
	var formatStr string
	err = c.RPC("StopCapturingAudio").Args(portID.Int()).Returns(&path, &formatStr).Call(ctx)
	if err != nil {
		return "", "", err
	}
	format = AudioSampleFormat(formatStr)
	return path, format, nil
}

// StartPlayingAudio calls the Chameleond RPC method of the same name.
// This implements Chameleond.StartPlayingAudio, see that for more details.
func (c *CommonChameleond) StartPlayingAudio(ctx context.Context, portID PortID, path string, format *AudioDataFormat) error {
	return c.RPC("StartPlayingAudio").Args(portID.Int(), path, format.Map()).Call(ctx)
}

// StartPlayingEcho calls the Chameleond RPC method of the same name.
// This implements Chameleond.StartPlayingEcho, see that for more details.
func (c *CommonChameleond) StartPlayingEcho(ctx context.Context, portID, inputID PortID) error {
	return c.RPC("StartPlayingEcho").Args(portID.Int(), inputID.Int()).Call(ctx)
}

// StopPlayingAudio calls the Chameleond RPC method of the same name.
// This implements Chameleond.StopPlayingAudio, see that for more details.
func (c *CommonChameleond) StopPlayingAudio(ctx context.Context, portID PortID) error {
	return c.RPC("StopPlayingAudio").Args(portID.Int()).Call(ctx)
}

// AudioBoardConnect calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardConnect, see that for more details.
func (c *CommonChameleond) AudioBoardConnect(ctx context.Context, busNumber AudioBusNumber, endpoint AudioBusEndpoint) error {
	return c.RPC("AudioBoardConnect").Args(busNumber.Int(), endpoint.String()).Call(ctx)
}

// AudioBoardDisconnect calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardDisconnect, see that for more details.
func (c *CommonChameleond) AudioBoardDisconnect(ctx context.Context, busNumber AudioBusNumber, endpoint AudioBusEndpoint) error {
	return c.RPC("AudioBoardDisconnect").Args(busNumber.Int(), endpoint.String()).Call(ctx)
}

// AudioBoardGetRoutes calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardGetRoutes, see that for more details.
func (c *CommonChameleond) AudioBoardGetRoutes(ctx context.Context, busNumber AudioBusNumber) ([]AudioBoardRoute, error) {
	var rawResult [][]interface{}
	err := c.RPC("AudioBoardGetRoutes").Args(busNumber.Int()).Call(ctx)
	if err != nil {
		return nil, err
	}
	routes := make([]AudioBoardRoute, len(rawResult))
	for i, statusTuple := range rawResult {
		if len(rawResult) != 2 {
			return nil, errors.Errorf("failed to unmarshall AudioBoardGetRoutes return, raw return value: %v", rawResult)
		}
		route := AudioBoardRoute{}
		source, ok := statusTuple[0].(string)
		if !ok {
			return nil, errors.Errorf("failed to unmarshall AudioBoardGetRoutes return, raw return value: %v", rawResult)
		}
		route.Source = AudioBusEndpoint(source)
		sink, ok := statusTuple[1].(string)
		if !ok {
			return nil, errors.Errorf("failed to unmarshall AudioBoardGetRoutes return, raw return value: %v", rawResult)
		}
		route.Sink = AudioBusEndpoint(sink)
		routes[i] = route
	}
	return routes, nil
}

// AudioBoardClearRoutes calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardClearRoutes, see that for more details.
func (c *CommonChameleond) AudioBoardClearRoutes(ctx context.Context, busNumber AudioBusNumber) error {
	return c.RPC("AudioBoardClearRoutes").Args(busNumber.Int()).Call(ctx)
}

// AudioBoardHasJackPlugger calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardHasJackPlugger, see that for more details.
func (c *CommonChameleond) AudioBoardHasJackPlugger(ctx context.Context) (bool, error) {
	return c.RPC("AudioBoardHasJackPlugger").CallForBool(ctx)
}

// AudioBoardAudioJackPlug calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardAudioJackPlug, see that for more details.
func (c *CommonChameleond) AudioBoardAudioJackPlug(ctx context.Context) error {
	return c.RPC("AudioBoardAudioJackPlug").Call(ctx)
}

// AudioBoardAudioJackUnplug calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardAudioJackUnplug, see that for more details.
func (c *CommonChameleond) AudioBoardAudioJackUnplug(ctx context.Context) error {
	return c.RPC("AudioBoardAudioJackUnplug").Call(ctx)
}

// SetUSBDriverPlaybackConfigs calls the Chameleond RPC method of the same name.
// This implements Chameleond.SetUSBDriverPlaybackConfigs, see that for more details.
func (c *CommonChameleond) SetUSBDriverPlaybackConfigs(ctx context.Context, playbackDataFormat *AudioDataFormat) error {
	return c.RPC("SetUSBDriverPlaybackConfigs").Args(playbackDataFormat.Map()).Call(ctx)
}

// SetUSBDriverCaptureConfigs calls the Chameleond RPC method of the same name.
// This implements Chameleond.SetUSBDriverCaptureConfigs, see that for more details.
func (c *CommonChameleond) SetUSBDriverCaptureConfigs(ctx context.Context, captureDataFormat *AudioDataFormat) error {
	return c.RPC("SetUSBDriverCaptureConfigs").Args(captureDataFormat.Map()).Call(ctx)
}

// AudioBoardResetBluetooth calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardResetBluetooth, see that for more details.
func (c *CommonChameleond) AudioBoardResetBluetooth(ctx context.Context) error {
	return c.RPC("AudioBoardResetBluetooth").Call(ctx)
}

// AudioBoardDisableBluetooth calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardDisableBluetooth, see that for more details.
func (c *CommonChameleond) AudioBoardDisableBluetooth(ctx context.Context) error {
	return c.RPC("AudioBoardDisableBluetooth").Call(ctx)
}

// AudioBoardIsBluetoothEnabled calls the Chameleond RPC method of the same name.
// This implements Chameleond.AudioBoardIsBluetoothEnabled, see that for more details.
func (c *CommonChameleond) AudioBoardIsBluetoothEnabled(ctx context.Context) (bool, error) {
	return c.RPC("AudioBoardIsBluetoothEnabled").CallForBool(ctx)
}

// ResetBluetoothRef calls the Chameleond RPC method of the same name.
// This implements Chameleond.ResetBluetoothRef, see that for more details.
func (c *CommonChameleond) ResetBluetoothRef(ctx context.Context) error {
	return c.RPC("ResetBluetoothRef").Call(ctx)
}

// DisableBluetoothRef calls the Chameleond RPC method of the same name.
// This implements Chameleond.DisableBluetoothRef, see that for more details.
func (c *CommonChameleond) DisableBluetoothRef(ctx context.Context) error {
	return c.RPC("DisableBluetoothRef").Call(ctx)
}

// IsBluetoothRefDisabled calls the Chameleond RPC method of the same name.
// This implements Chameleond.IsBluetoothRefDisabled, see that for more details.
func (c *CommonChameleond) IsBluetoothRefDisabled(ctx context.Context) (bool, error) {
	return c.RPC("IsBluetoothRefDisabled").CallForBool(ctx)
}

// TriggerLinkFailure calls the Chameleond RPC method of the same name.
// This implements Chameleond.TriggerLinkFailure, see that for more details.
func (c *CommonChameleond) TriggerLinkFailure(ctx context.Context, portID PortID) error {
	return c.RPC("TriggerLinkFailure").Args(portID.Int()).Call(ctx)
}

// HasVideoSupport calls the Chameleond RPC method of the same name.
// This implements Chameleond.HasVideoSupport, see that for more details.
func (c *CommonChameleond) HasVideoSupport(ctx context.Context, portID PortID) (bool, error) {
	return c.RPC("HasVideoSupport").Args(portID.Int()).CallForBool(ctx)
}

// SetVgaMode calls the Chameleond RPC method of the same name.
// This implements Chameleond.SetVgaMode, see that for more details.
func (c *CommonChameleond) SetVgaMode(ctx context.Context, portID PortID, mode string) error {
	return c.RPC("SetVgaMode").Args(portID.Int(), mode).Call(ctx)
}

// WaitVideoInputStable calls the Chameleond RPC method of the same name.
// This implements Chameleond.WaitVideoInputStable, see that for more details.
func (c *CommonChameleond) WaitVideoInputStable(ctx context.Context, portID PortID, timeoutSeconds float64) (stableBeforeTimeout bool, err error) {
	return c.RPC("WaitVideoInputStable").Args(portID.Int(), timeoutSeconds).CallForBool(ctx)
}

// CreateEdid calls the Chameleond RPC method of the same name.
// This implements Chameleond.CreateEdid, see that for more details.
func (c *CommonChameleond) CreateEdid(ctx context.Context, edid []byte) (edidID int, err error) {
	return c.RPC("CreateEdid").Args(edid).CallForInt(ctx)
}

// DestroyEdid calls the Chameleond RPC method of the same name.
// This implements Chameleond.DestroyEdid, see that for more details.
func (c *CommonChameleond) DestroyEdid(ctx context.Context, edidID int) error {
	return c.RPC("DestroyEdid").Args(edidID).Call(ctx)
}

// SetDdcState calls the Chameleond RPC method of the same name.
// This implements Chameleond.SetDdcState, see that for more details.
func (c *CommonChameleond) SetDdcState(ctx context.Context, portID PortID, enabled bool) error {
	return c.RPC("SetDdcState").Args(portID.Int(), enabled).Call(ctx)
}

// IsDdcEnabled calls the Chameleond RPC method of the same name.
// This implements Chameleond.IsDdcEnabled, see that for more details.
func (c *CommonChameleond) IsDdcEnabled(ctx context.Context, portID PortID) (bool, error) {
	return c.RPC("IsDdcEnabled").Args(portID.Int()).CallForBool(ctx)
}

// ReadEdid calls the Chameleond RPC method of the same name.
// This implements Chameleond.ReadEdid, see that for more details.
func (c *CommonChameleond) ReadEdid(ctx context.Context, portID PortID) (edid []byte, err error) {
	return c.RPC("ReadEdid").Args(portID.Int()).CallForBytes(ctx)
}

// ApplyEdid calls the Chameleond RPC method of the same name.
// This implements Chameleond.ApplyEdid, see that for more details.
func (c *CommonChameleond) ApplyEdid(ctx context.Context, portID PortID, edidID int) error {
	return c.RPC("ApplyEdid").Args(portID.Int(), edidID).Call(ctx)
}

// UnplugHPD calls the Chameleond RPC method of the same name.
// This implements Chameleond.UnplugHPD, see that for more details.
func (c *CommonChameleond) UnplugHPD(ctx context.Context, portID PortID) error {
	return c.RPC("UnplugHPD").Args(portID.Int()).Call(ctx)
}

// FireHpdPulse calls the Chameleond RPC method of the same name.
// This implements Chameleond.FireHpdPulse, see that for more details.
func (c *CommonChameleond) FireHpdPulse(ctx context.Context, portID PortID, deAssertIntervalMicroSec, assertIntervalMicroSec, repeatCount, endLevel int) error {
	return c.RPC("FireHpdPulse").Args(portID.Int(), deAssertIntervalMicroSec, assertIntervalMicroSec, repeatCount, endLevel).Call(ctx)
}

// FireMixedHpdPulses calls the Chameleond RPC method of the same name.
// This implements Chameleond.FireMixedHpdPulses, see that for more details.
func (c *CommonChameleond) FireMixedHpdPulses(ctx context.Context, portID PortID, widthsMsec []int) error {
	return c.RPC("FireMixedHpdPulses").Args(portID.Int(), widthsMsec).Call(ctx)
}

// ScheduleHpdToggle calls the Chameleond RPC method of the same name.
// This implements Chameleond.ScheduleHpdToggle, see that for more details.
func (c *CommonChameleond) ScheduleHpdToggle(ctx context.Context, portID PortID, delayMs int, risingEdge bool) error {
	return c.RPC("ScheduleHpdToggle").Args(portID.Int(), delayMs, risingEdge).Call(ctx)
}

// SetContentProtection calls the Chameleond RPC method of the same name.
// This implements Chameleond.SetContentProtection, see that for more details.
func (c *CommonChameleond) SetContentProtection(ctx context.Context, portID PortID, enabled bool) error {
	return c.RPC("SetContentProtection").Args(portID.Int(), enabled).Call(ctx)
}

// IsContentProtectionEnabled calls the Chameleond RPC method of the same name.
// This implements Chameleond.IsContentProtectionEnabled, see that for more details.
func (c *CommonChameleond) IsContentProtectionEnabled(ctx context.Context, portID PortID) (bool, error) {
	return c.RPC("IsContentProtectionEnabled").Args(portID.Int()).CallForBool(ctx)
}

// IsVideoInputEncrypted calls the Chameleond RPC method of the same name.
// This implements Chameleond.IsVideoInputEncrypted, see that for more details.
func (c *CommonChameleond) IsVideoInputEncrypted(ctx context.Context, portID PortID) (bool, error) {
	return c.RPC("IsVideoInputEncrypted").Args(portID.Int()).CallForBool(ctx)
}

// DumpPixels calls the Chameleond RPC method of the same name.
// This implements Chameleond.DumpPixels, see that for more details.
func (c *CommonChameleond) DumpPixels(ctx context.Context, portID PortID, view *ImageView) error {
	args := []interface{}{
		portID.Int(),
	}
	if view != nil {
		args = append(args, view.X)
		args = append(args, view.Y)
		args = append(args, view.Width)
		args = append(args, view.Height)
	}
	return c.RPC("DumpPixels").Args(args...).Call(ctx)
}

// GetMaxFrameLimit calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetMaxFrameLimit, see that for more details.
func (c *CommonChameleond) GetMaxFrameLimit(ctx context.Context, portID PortID, width, height int) (int, error) {
	return c.RPC("GetMaxFrameLimit").Args(portID.Int(), width, height).CallForInt(ctx)
}

// StartCapturingVideo calls the Chameleond RPC method of the same name.
// This implements Chameleond.StartCapturingVideo, see that for more details.
func (c *CommonChameleond) StartCapturingVideo(ctx context.Context, portID PortID, view *ImageView) error {
	args := []interface{}{
		portID.Int(),
	}
	if view != nil {
		args = append(args, view.X)
		args = append(args, view.Y)
		args = append(args, view.Width)
		args = append(args, view.Height)
	}
	return c.RPC("StartCapturingVideo").Args(args...).Call(ctx)
}

// StopCapturingVideo calls the Chameleond RPC method of the same name.
// This implements Chameleond.StopCapturingVideo, see that for more details.
func (c *CommonChameleond) StopCapturingVideo(ctx context.Context, stopIndex int) error {
	return c.RPC("StopCapturingVideo").Args(stopIndex).Call(ctx)
}

// CaptureVideo calls the Chameleond RPC method of the same name.
// This implements Chameleond.CaptureVideo, see that for more details.
func (c *CommonChameleond) CaptureVideo(ctx context.Context, portID PortID, totalFrame int, view *ImageView) error {
	args := []interface{}{
		portID.Int(),
		totalFrame,
	}
	if view != nil {
		args = append(args, view.X)
		args = append(args, view.Y)
		args = append(args, view.Width)
		args = append(args, view.Height)
	}
	return c.RPC("CaptureVideo").Args(args...).Call(ctx)
}

// GetCapturedFrameCount calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetCapturedFrameCount, see that for more details.
func (c *CommonChameleond) GetCapturedFrameCount(ctx context.Context) (int, error) {
	return c.RPC("GetCapturedFrameCount").CallForInt(ctx)
}

// GetCapturedResolution calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetCapturedResolution, see that for more details.
func (c *CommonChameleond) GetCapturedResolution(ctx context.Context) (width, height int, err error) {
	err = c.RPC("GetCapturedResolution").Returns(&width, &height).Call(ctx)
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

// ReadCapturedFrame calls the Chameleond RPC method of the same name.
// This implements Chameleond.ReadCapturedFrame, see that for more details.
func (c *CommonChameleond) ReadCapturedFrame(ctx context.Context, frameIndex int) (pixels []byte, err error) {
	return c.RPC("ReadCapturedFrame").Args(frameIndex).CallForBytes(ctx)
}

// CacheFrameThumbnail calls the Chameleond RPC method of the same name.
// This implements Chameleond.CacheFrameThumbnail, see that for more details.
func (c *CommonChameleond) CacheFrameThumbnail(ctx context.Context, frameIndex, ratio int) (thumbnailID int, err error) {
	return c.RPC("CacheFrameThumbnail").Args(frameIndex, ratio).CallForInt(ctx)
}

// GetCapturedChecksums calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetCapturedChecksums, see that for more details.
func (c *CommonChameleond) GetCapturedChecksums(ctx context.Context, startIndex, stopIndex int) (checksums []uint32, err error) {
	err = c.RPC("GetCapturedChecksums").Args(startIndex, stopIndex).Returns(&checksums).Call(ctx)
	if err != nil {
		return nil, err
	}
	return checksums, nil
}

// GetCapturedHistograms calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetCapturedHistograms, see that for more details.
func (c *CommonChameleond) GetCapturedHistograms(ctx context.Context, startIndex, stopIndex int) (histograms [][]float64, err error) {
	err = c.RPC("GetCapturedHistograms").Args(startIndex, stopIndex).Returns(&histograms).Call(ctx)
	if err != nil {
		return nil, err
	}
	return histograms, nil
}

// ComputePixelChecksum calls the Chameleond RPC method of the same name.
// This implements Chameleond.ComputePixelChecksum, see that for more details.
func (c *CommonChameleond) ComputePixelChecksum(ctx context.Context, portID PortID, view *ImageView) (checksum uint32, err error) {
	args := []interface{}{
		portID.Int(),
	}
	if view != nil {
		args = append(args, view.X)
		args = append(args, view.Y)
		args = append(args, view.Width)
		args = append(args, view.Height)
	}
	err = c.RPC("ComputePixelChecksum").Args(args...).Returns(&checksum).Call(ctx)
	if err != nil {
		return 0, err
	}
	return checksum, nil
}

// DetectResolution calls the Chameleond RPC method of the same name.
// This implements Chameleond.DetectResolution, see that for more details.
func (c *CommonChameleond) DetectResolution(ctx context.Context, portID PortID) (width, height int, err error) {
	err = c.RPC("DetectResolution").Args(portID.Int()).Returns(&width, &height).Call(ctx)
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

// GetVideoParams calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetVideoParams, see that for more details.
func (c *CommonChameleond) GetVideoParams(ctx context.Context, portID PortID) (*VideoParams, error) {
	paramsMap := make(map[string]interface{})
	err := c.RPC("GetVideoParams").Args(portID.Int()).Returns(paramsMap).Call(ctx)
	if err != nil {
		return nil, err
	}
	return MapToVideoParams(paramsMap)
}

// GetLastInfoFrame calls the Chameleond RPC method of the same name.
// This implements Chameleond.GetLastInfoFrame, see that for more details.
func (c *CommonChameleond) GetLastInfoFrame(ctx context.Context, portID PortID, infoFrameType InfoFrameType) (version int, payload []byte, length int, err error) {
	err = c.RPC("GetLastInfoFrame").Args(portID.Int(), infoFrameType.String()).Returns(&version, &payload, &length).Call(ctx)
	if err != nil {
		return 0, nil, 0, err
	}
	return version, payload, length, err
}

// AudioBoardDevice returns an RPC interface for making RPC calls to the
// chameleon audio board device.
func (c *CommonChameleond) AudioBoardDevice() devices.ChameleonDevice {
	return c.audioBoardDevice
}

// AVSyncProbeDevice returns an RPC interface for making RPC calls to the
// chameleon AV sync probe device.
func (c *CommonChameleond) AVSyncProbeDevice() devices.ChameleonDevice {
	return c.aVSyncProbeDevice
}

// BLEFastPair returns an RPC interface for making RPC calls to the
// chameleon bluetooth LE fast pair device.
func (c *CommonChameleond) BLEFastPair() bluetooth.FastPairPeripheral {
	return c.bLEFastPair
}

// BLEKeyboard returns an RPC interface for making RPC calls to the
// chameleon bluetooth LE keyboard device.
func (c *CommonChameleond) BLEKeyboard() bluetooth.KeyboardPeripheral {
	return c.bLEKeyboard
}

// BLEMouse returns an RPC interface for making RPC calls to the
// chameleon bluetooth LE mouse device.
func (c *CommonChameleond) BLEMouse() bluetooth.MousePeripheral {
	return c.bLEMouse
}

// BLEPhone returns an RPC interface for making RPC calls to the
// chameleon bluetooth LE phone device.
func (c *CommonChameleond) BLEPhone() bluetooth.PhonePeripheral {
	return c.bLEPhone
}

// BluetoothA2DPSink returns an RPC interface for making RPC calls to the
// chameleon bluetooth A2DP sink device.
func (c *CommonChameleond) BluetoothA2DPSink() devices.ChameleonDevice {
	return c.bluetoothA2DPSink
}

// BluetoothAudioDevice returns an XMLRPC interface for making XMLRPC calls
// to the chameleon bluetooth audio device.
func (c *CommonChameleond) BluetoothAudioDevice() bluetooth.AudioPeripheral {
	return c.bluetoothAudioDevice
}

// BluetoothBaseDevice returns an XMLRPC interface for making XMLRPC calls to
// the chameleon bluetooth base device.
func (c *CommonChameleond) BluetoothBaseDevice() bluetooth.BasePeripheral {
	return c.bluetoothBaseDevice
}

// BluetoothKeyboardDevice returns an XMLRPC interface for making XMLRPC
// calls to the chameleon bluetooth keyboard device.
func (c *CommonChameleond) BluetoothKeyboardDevice() bluetooth.KeyboardPeripheral {
	return c.bluetoothKeyboardDevice
}

// BluetoothMouseDevice returns an XMLRPC interface for making XMLRPC calls
// to the chameleon bluetooth mouse device.
func (c *CommonChameleond) BluetoothMouseDevice() bluetooth.MousePeripheral {
	return c.bluetoothMouseDevice
}

// BluetoothTesterDevice returns an XMLRPC interface for making XMLRPC calls
// to the chameleon bluetooth tester device.
func (c *CommonChameleond) BluetoothTesterDevice() devices.ChameleonDevice {
	return c.bluetoothTesterDevice
}

// MotorBoardDevice returns an XMLRPC interface for making XMLRPC calls to
// the chameleon motor board device.
func (c *CommonChameleond) MotorBoardDevice() devices.ChameleonDevice {
	return c.motorBoardDevice
}

// PrinterDevice returns an XMLRPC interface for making XMLRPC calls to the
// chameleon printer device.
func (c *CommonChameleond) PrinterDevice() devices.ChameleonDevice {
	return c.printerDevice
}

// FetchSupportedPortIDsByType returns all supported port ids (PortID) mapped
// by their connector type (PortType).
//
// This is not a direct Chameleond RPC method call, but does make calls to
// RPC methods to obtain this information.
//
// Supported PortID values are identified by the result of calling
// GetSupportedPorts and each PortType is identified by calling
// GetConnectorType on each PortID. Multiple PortID values may have the same
// PortType, so the values of the map are sorted PortID arrays.
//
// Results are cached the first time FetchSupportedPortIDsByType is called.
// Subsequent calls will make no new RPC calls and simply return the same cached
// map.
func (c *CommonChameleond) FetchSupportedPortIDsByType(ctx context.Context) (map[PortType][]PortID, error) {
	if c.supportedPortIDsByType != nil {
		return c.supportedPortIDsByType, nil
	}
	// Get and sort supported port IDs.
	supportedPortIDs, err := c.GetSupportedPorts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get supported ports")
	}
	sort.Slice(supportedPortIDs, func(i, j int) bool {
		return supportedPortIDs[i].Int() < supportedPortIDs[j].Int()
	})
	// Map each port ID to its type.
	supportedPortIDsByType := make(map[PortType][]PortID)
	for _, portID := range supportedPortIDs {
		portType, err := c.GetConnectorType(ctx, portID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get port type of port with id %d", portID)
		}
		supportedPortIDsByType[portType] = append(supportedPortIDsByType[portType], portID)
	}
	c.supportedPortIDsByType = supportedPortIDsByType
	testing.ContextLogf(ctx, "Cached Chameleond supported port ids by type for device %q as %v", c.Host(), c.supportedPortIDsByType)
	return supportedPortIDsByType, nil
}

// FetchSupportedPortTypes returns a sorted list of all PortType values that
// this device supports.
//
// This is not a direct Chameleond RPC method call, but does make calls to
// RPC methods to obtain this information.
func (c *CommonChameleond) FetchSupportedPortTypes(ctx context.Context) ([]PortType, error) {
	supportedPortIDsByType, err := c.FetchSupportedPortIDsByType(ctx)
	if err != nil {
		return nil, err
	}
	return c.collectSupportedPortTypes(supportedPortIDsByType), nil
}

func (c *CommonChameleond) collectSupportedPortTypes(supportedPortIDsByType map[PortType][]PortID) []PortType {
	supportedPortTypes := make([]PortType, 0)
	for portType := range supportedPortIDsByType {
		supportedPortTypes = append(supportedPortTypes, portType)
	}
	sort.Slice(supportedPortTypes, func(i, j int) bool {
		return supportedPortTypes[i].String() < supportedPortTypes[j].String()
	})
	return supportedPortTypes
}

// FetchSupportedPortIDByType returns the device-specific integer ID of the port with
// the given PortType and index. The PortType matching is case-insensitive.
//
// This is not a direct Chameleond RPC method call, but does make calls to
// RPC methods to obtain this information.
func (c *CommonChameleond) FetchSupportedPortIDByType(ctx context.Context, portType PortType, index int) (PortID, error) {
	supportedPortIDsByType, err := c.FetchSupportedPortIDsByType(ctx)
	if err != nil {
		return 0, err
	}
	supportedPortTypes := c.collectSupportedPortTypes(supportedPortIDsByType)
	var matchingPortType PortType
	for _, supportedPortType := range supportedPortTypes {
		if strings.EqualFold(supportedPortType.String(), portType.String()) {
			matchingPortType = supportedPortType
			break
		}
	}
	if matchingPortType == "" {
		return 0, errors.Errorf("this device does not support ports of type %q; supported port types for this device are %v", portType, supportedPortTypes)
	}
	portIDs := supportedPortIDsByType[matchingPortType]
	if index < 0 || index >= len(portIDs) {
		return 0, errors.Errorf(
			"invalid port index %d, this device supports %d ports of type %q so index values are of the range 0 to %d, inclusive",
			index, len(portIDs), portType, len(portIDs)-1)
	}
	return portIDs[index], nil
}
