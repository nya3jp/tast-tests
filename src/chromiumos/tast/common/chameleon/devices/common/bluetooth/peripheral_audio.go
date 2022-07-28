// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
)

// AudioPeripheral is an interface for making RPC calls to a chameleond daemon
// targeting a specific bluetooth audio peripheral chameleon device flow.
//
// This is based off of the Python class "chameleond.utils.bluetooth_audio.BluetoothAudio"
// from the chameleon source. Refer to that source for more complete
// documentation.
type AudioPeripheral interface {
	BluezPeripheral

	// StartPulseaudio calls the Chameleond RPC method of the same name.
	// Starts the pulseaudio process.
	// Returns true if successful.
	StartPulseaudio(ctx context.Context, audioProfile string) (bool, error)

	// StopPulseaudio calls the Chameleond RPC method of the same name.
	// Stops the pulseaudio process.
	// Returns true if successful.
	StopPulseaudio(ctx context.Context) (bool, error)

	// StartOfono calls the Chameleond RPC method of the same name.
	// Starts/restarts the Ofono process.
	// Returns true if successful.
	StartOfono(ctx context.Context) (bool, error)

	// StopOfono calls the Chameleond RPC method of the same name.
	// Stops the Ofono process.
	// Returns true if successful.
	StopOfono(ctx context.Context) (bool, error)

	// PlayAudio calls the Chameleond RPC method of the same name.
	// Plays the audio file located at audioFile on the chameleon device.
	PlayAudio(ctx context.Context, audioFile string) (bool, error)

	// StartPlayingAudioSubprocess calls the Chameleond RPC method of the same name.
	// Starts playing the audio file in a subprocess.
	StartPlayingAudioSubprocess(ctx context.Context, audioProfile string, testData map[string]string, waitSecs int) (bool, error)

	// StopPlayingAudioSubprocess calls the Chameleond RPC method of the same name.
	// Stops playing the audio file in the subprocess.
	StopPlayingAudioSubprocess(ctx context.Context) (bool, error)

	// ListCards calls the Chameleond RPC method of the same name.
	// Lists all sound cards.
	// Returns the pactl command call output as a string.
	ListCards(ctx context.Context, audioProfile string) (string, error)

	// ListSources calls the Chameleond RPC method of the same name.
	// Lists all audio source cards.
	// Returns the pactl command call output as a string.
	ListSources(ctx context.Context, audioProfile string) (string, error)

	// ListSinks calls the Chameleond RPC method of the same name.
	// Lists all audio sinks.
	// Returns the pactl command call output as a string.
	ListSinks(ctx context.Context, audioProfile string) (string, error)

	// GetBluezSourceDevice calls the Chameleond RPC method of the same name.
	// Gets the number of the bluez source device.
	GetBluezSourceDevice(ctx context.Context, audioProfile string) (string, error)

	// GetBluezSourceHFPDevice calls the Chameleond RPC method of the same name.
	// Gets the number of the HFP bluez source device.
	GetBluezSourceHFPDevice(ctx context.Context, audioProfile string) (int, error)

	// GetBluezSinkHFPDevice calls the Chameleond RPC method of the same name.
	// Gets the number of the HFB bluez sink device.
	GetBluezSinkHFPDevice(ctx context.Context, audioProfile string) (int, error)

	// GetBluezSourceA2DPDevice calls the Chameleond RPC method of the same name.
	// Gets the number of the A2DP bluez sink device.
	GetBluezSourceA2DPDevice(ctx context.Context, audioProfile string) (int, error)

	// StartRecordingAudioSubprocess calls the Chameleond RPC method of the same name.
	// Starts recording audio in a subprocess.
	// Returns true if successful.
	StartRecordingAudioSubprocess(ctx context.Context, audioProfile string, testData map[string]string, recordingEntity string) (bool, error)

	// HandleOneChunk calls the Chameleond RPC method of the same name.
	// Saves one chunk of data into a file and remote copies it to the DUT.
	// Returns the chuck filename if successful.
	HandleOneChunk(ctx context.Context, chunkInSecs, index int, dutIP string) (string, error)

	// StopRecordingingAudioSubprocess calls the Chameleond RPC method of the same name.
	// Stops the recording subprocess.
	// Returns true if successful.
	//
	// Note that the typo in the function name is kept as the RPC method also has
	// the same typo.
	StopRecordingingAudioSubprocess(ctx context.Context) (string, error)

	// ScpToDut calls the Chameleond RPC method of the same name.
	// Copies the srcFile to the dstFile via scp at dutIP.
	ScpToDut(ctx context.Context, srcFile, dstFile, dutIP string) error

	// ExportMediaPlayer calls the Chameleond RPC method of the same name.
	// Exports the Bluetooth media player.
	// Returns true if one and only one mpris-proxy is running.
	ExportMediaPlayer(ctx context.Context) (bool, error)

	// UnexportMediaPlayer calls the Chameleond RPC method of the same name.
	// Stops all mpris-proxy processes.
	UnexportMediaPlayer(ctx context.Context) (bool, error)

	// GetExportedMediaPlayer calls the Chameleond RPC method of the same name.
	// Gets the exported media player's name with playerctl.
	GetExportedMediaPlayer(ctx context.Context) (string, error)

	// SendMediaPlayerCommand calls the Chameleond RPC method of the same name.
	// Executes the command towards the given player.
	// Returns true if successful.
	SendMediaPlayerCommand(ctx context.Context, command string) (bool, error)

	// GetMediaPlayerMediaInfo calls the Chameleond RPC method of the same name.
	// Retrieves media information through playerctl calls.
	GetMediaPlayerMediaInfo(ctx context.Context) (map[string]string, error)
}

// CommonAudioPeripheral is a base implementation of AudioPeripheral that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the AudioPeripheral interface for more detailed documentation.
type CommonAudioPeripheral struct {
	CommonBluezPeripheral
}

// NewCommonAudioPeripheral creates a new instance of CommonAudioPeripheral.
func NewCommonAudioPeripheral(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonAudioPeripheral {
	return &CommonAudioPeripheral{
		CommonBluezPeripheral: *NewCommonBluezPeripheral(xmlrpcClient, methodNamePrefix),
	}
}

// StartPulseaudio calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.StartPulseaudio, see that for more details.
func (c *CommonAudioPeripheral) StartPulseaudio(ctx context.Context, audioProfile string) (bool, error) {
	return c.RPC("StartPulseaudio").Args(audioProfile).CallForBool(ctx)
}

// StopPulseaudio calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.StopPulseaudio, see that for more details.
func (c *CommonAudioPeripheral) StopPulseaudio(ctx context.Context) (bool, error) {
	return c.RPC("StopPulseaudio").CallForBool(ctx)
}

// StartOfono calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.StartOfono, see that for more details.
func (c *CommonAudioPeripheral) StartOfono(ctx context.Context) (bool, error) {
	return c.RPC("StartOfono").CallForBool(ctx)
}

// StopOfono calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.StopOfono, see that for more details.
func (c *CommonAudioPeripheral) StopOfono(ctx context.Context) (bool, error) {
	return c.RPC("StopOfono").CallForBool(ctx)
}

// PlayAudio calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.PlayAudio, see that for more details.
func (c *CommonAudioPeripheral) PlayAudio(ctx context.Context, audioFile string) (bool, error) {
	return c.RPC("PlayAudio").Args(audioFile).CallForBool(ctx)
}

// StartPlayingAudioSubprocess calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.StartPlayingAudioSubprocess, see that for
// more details.
func (c *CommonAudioPeripheral) StartPlayingAudioSubprocess(ctx context.Context, audioProfile string, testData map[string]string, waitSecs int) (bool, error) {
	return c.RPC("StartPlayingAudioSubprocess").Args(audioProfile).CallForBool(ctx)
}

// StopPlayingAudioSubprocess calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.StopPlayingAudioSubprocess, see that for more
// details.
func (c *CommonAudioPeripheral) StopPlayingAudioSubprocess(ctx context.Context) (bool, error) {
	return c.RPC("StopPlayingAudioSubprocess").CallForBool(ctx)
}

// ListCards calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.ListCards, see that for more details.
func (c *CommonAudioPeripheral) ListCards(ctx context.Context, audioProfile string) (string, error) {
	return c.RPC("ListCards").Args(audioProfile).CallForString(ctx)
}

// ListSources calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.ListSources, see that for more details.
func (c *CommonAudioPeripheral) ListSources(ctx context.Context, audioProfile string) (string, error) {
	return c.RPC("ListSources").Args(audioProfile).CallForString(ctx)
}

// ListSinks calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.ListSinks, see that for more details.
func (c *CommonAudioPeripheral) ListSinks(ctx context.Context, audioProfile string) (string, error) {
	return c.RPC("ListSinks").Args(audioProfile).CallForString(ctx)
}

// GetBluezSourceDevice calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.GetBluezSourceDevice, see that for more
// details.
func (c *CommonAudioPeripheral) GetBluezSourceDevice(ctx context.Context, audioProfile string) (string, error) {
	return c.RPC("GetBluezSourceDevice").Args(audioProfile).CallForString(ctx)
}

// GetBluezSourceHFPDevice calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.GetBluezSourceHFPDevice, see that for more
// details.
func (c *CommonAudioPeripheral) GetBluezSourceHFPDevice(ctx context.Context, audioProfile string) (int, error) {
	return c.RPC("GetBluezSourceHFPDevice").Args(audioProfile).CallForInt(ctx)
}

// GetBluezSinkHFPDevice calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.GetBluezSinkHFPDevice, see that for more
// details.
func (c *CommonAudioPeripheral) GetBluezSinkHFPDevice(ctx context.Context, audioProfile string) (int, error) {
	return c.RPC("GetBluezSinkHFPDevice").Args(audioProfile).CallForInt(ctx)
}

// GetBluezSourceA2DPDevice calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.GetBluezSourceA2DPDevice, see that for more
// details.
func (c *CommonAudioPeripheral) GetBluezSourceA2DPDevice(ctx context.Context, audioProfile string) (int, error) {
	return c.RPC("GetBluezSourceA2DPDevice").Args(audioProfile).CallForInt(ctx)
}

// StartRecordingAudioSubprocess calls the Chameleond RPC method of the same
// name. This implements AudioPeripheral.StartRecordingAudioSubprocess, see that
// for more details.
func (c *CommonAudioPeripheral) StartRecordingAudioSubprocess(ctx context.Context, audioProfile string, testData map[string]string, recordingEntity string) (bool, error) {
	return c.RPC("StartRecordingAudioSubprocess").Args(audioProfile, testData, recordingEntity).CallForBool(ctx)
}

// HandleOneChunk calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.HandleOneChunk, see that for more details.
func (c *CommonAudioPeripheral) HandleOneChunk(ctx context.Context, chunkInSecs, index int, dutIP string) (string, error) {
	return c.RPC("HandleOneChunk").Args(chunkInSecs, index, dutIP).CallForString(ctx)
}

// StopRecordingingAudioSubprocess calls the Chameleond RPC method of the same
// name. This implements AudioPeripheral.StopRecordingingAudioSubprocess, see
// that for more details.
func (c *CommonAudioPeripheral) StopRecordingingAudioSubprocess(ctx context.Context) (string, error) {
	return c.RPC("StopRecordingingAudioSubprocess").CallForString(ctx)
}

// ScpToDut calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.ScpToDut, see that for more details.
func (c *CommonAudioPeripheral) ScpToDut(ctx context.Context, srcFile, dstFile, dutIP string) error {
	return c.RPC("ScpToDut").Args(srcFile, dstFile, dutIP).Call(ctx)
}

// ExportMediaPlayer calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.ExportMediaPlayer, see that for more details.
func (c *CommonAudioPeripheral) ExportMediaPlayer(ctx context.Context) (bool, error) {
	return c.RPC("ExportMediaPlayer").CallForBool(ctx)
}

// UnexportMediaPlayer calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.UnexportMediaPlayer, see that for more
// details.
func (c *CommonAudioPeripheral) UnexportMediaPlayer(ctx context.Context) (bool, error) {
	return c.RPC("UnexportMediaPlayer").CallForBool(ctx)
}

// GetExportedMediaPlayer calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.GetExportedMediaPlayer, see that for more
// details.
func (c *CommonAudioPeripheral) GetExportedMediaPlayer(ctx context.Context) (string, error) {
	return c.RPC("GetExportedMediaPlayer").CallForString(ctx)
}

// SendMediaPlayerCommand calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.SendMediaPlayerCommand, see that for more
// details.
func (c *CommonAudioPeripheral) SendMediaPlayerCommand(ctx context.Context, command string) (bool, error) {
	return c.RPC("SendMediaPlayerCommand").Args(command).CallForBool(ctx)
}

// GetMediaPlayerMediaInfo calls the Chameleond RPC method of the same name.
// This implements AudioPeripheral.GetMediaPlayerMediaInfo, see that for more
// details.
func (c *CommonAudioPeripheral) GetMediaPlayerMediaInfo(ctx context.Context) (map[string]string, error) {
	var mediaPlayerInfo map[string]string
	err := c.RPC("GetMediaPlayerMediaInfo").Returns(&mediaPlayerInfo).Call(ctx)
	if err != nil {
		return nil, err
	}
	return mediaPlayerInfo, nil
}
