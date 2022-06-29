// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chameleon is used to communicate with chameleon devices connected to DUTs.
// It communicates with chameleon over XML-RPC.
package chameleon

import (
	"context"
	"reflect"

	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
)

// HasAudioSupport returns true if the port has audio support.
// portID: The ID of the input/output port.
func (ch *Chameleon) HasAudioSupport(ctx context.Context, portID PortID) (bool, error) {
	var hasAudioSupport bool
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("HasAudioSupport", portID), &hasAudioSupport); err != nil {
		return false, errors.Wrapf(err, "failed to make HasAudioSupport xmlrpc request with args portID: %v", portID)
	}
	return hasAudioSupport, nil
}

// AudioBoardConnect connects an endpoint to an audio bus.
// busNumber: 1 or 2 for audio bus 1 or bus 2.
// audioBusEndpoint defined by type AudioBusEndpoint
func (ch *Chameleon) AudioBoardConnect(ctx context.Context, busNumber AudioBusNumber, audioBusEndpoint AudioBusEndpoint) error {
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("AudioBoardConnect", busNumber, audioBusEndpoint)); err != nil {
		return errors.Wrapf(err, "failed to make AudioBoardConnect xmlrpc request with args busNumber: %v audioBusEndpoint: %v", busNumber, audioBusEndpoint)
	}
	return nil
}

// AudioBoardDisconnect disconnects an endpoint to an audio bus.
// busNumber: 1 or 2 for audio bus 1 or bus 2.
// audioBusEndpoint defined by type AudioBusEndpoint
func (ch *Chameleon) AudioBoardDisconnect(ctx context.Context, busNumber AudioBusNumber, audioBusEndpoint AudioBusEndpoint) error {
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("AudioBoardDisconnect", busNumber, audioBusEndpoint)); err != nil {
		return errors.Wrapf(err, "failed to make AudioBoardDisconnect xmlrpc request with args busNumber: %v audioBusEndpoint: %v", busNumber, audioBusEndpoint)
	}
	return nil
}

// StartPlayingAudio plays audio data at given path using given format and port.
//
// portID: The ID of the output connector.
// path: The path to the audio data to play.
// audioDataFormat: A map representation of AudioDataFormat. Currently Chameleon only accepts the following data format:
//
//	    var SUPPORTED_AUDIO_DATA_FORMAT = map[string]interface{}{
//	    	"file_type":     RAW,
//		    "sample_format": S32_LE,
//		    "channel":       8,
//		    "rate":          48000,
//	    }
//
// User should do the format conversion to minimize workload on Chameleon board.
func (ch *Chameleon) StartPlayingAudio(ctx context.Context, portID PortID, path string, audioDataFormat map[string]interface{}) error {
	if !reflect.DeepEqual(audioDataFormat, SupportedAudioDataFormat) {
		return errors.Errorf("unsupported audio playback format: %v. The only supported format is: %v", audioDataFormat, SupportedAudioDataFormat)
	}

	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("StartPlayingAudio", portID, path, audioDataFormat)); err != nil {
		return errors.Wrapf(err, "failed to make StartPlayingAudio xmlrpc request with args portID: %v path: %s audioDataFormat: %v", portID, path, audioDataFormat)
	}
	return nil
}

// StopPlayingAudio stops playing audio from port_id port.
// port_id: The ID of the output connector.
func (ch *Chameleon) StopPlayingAudio(ctx context.Context, portID PortID) error {
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("StopPlayingAudio", portID)); err != nil {
		return errors.Wrapf(err, "failed to make StopPlayingAudio xmlrpc request with args portID: %v", portID)
	}
	return nil
}

// GetAudioChannelMapping obtains the channel mapping for an audio port.
// Audio channels are not guaranteed to not be swapped. Clients can use the
// channel mapping to match a wire channel to a Chameleon channel.
// This function may only be called when audio capture or playback is in
// progress.
//
//	port_id: The ID of the audio port.
//
// Returns:
//
//	An array of integers. There is one element per Chameleon channel.
//	For audio input ports, each element indicates which input channel the
//	capture channel is mapped to. For audio output ports, each element
//	indicates which output channel the playback channel is mapped to. As a
//	special case, -1 means the channel isn't mapped.
func (ch *Chameleon) GetAudioChannelMapping(ctx context.Context, portID PortID) ([]int, error) {
	var value []int
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("GetAudioChannelMapping", portID), &value); err != nil {
		return value, errors.Wrapf(err, "failed to make GetAudioChannelMapping xmlrpc request with args portID: %v", portID)
	}
	return value, nil
}

// AudioBoardGetRoutes gets a list of routes on audio bus.
// busNumber: 1 or 2 for audio bus 1 or bus 2.
// returns array of [source, sink] that are routed on audio bus
// where source and sink are endpoints defined in AudioBusEndpoint.
func (ch *Chameleon) AudioBoardGetRoutes(ctx context.Context, busNumber AudioBusNumber) ([][]string, error) {
	var value [][]string
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("AudioBoardGetRoutes", busNumber), &value); err != nil {
		return value, errors.Wrapf(err, "failed to make AudioBoardGetRoutes xmlrpc request with args busNumber: %v", busNumber)
	}
	return value, nil
}

// AudioBoardClearRoutes clears routes on an audio bus.
func (ch *Chameleon) AudioBoardClearRoutes(ctx context.Context, busNumber AudioBusNumber) error {
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("AudioBoardClearRoutes", busNumber)); err != nil {
		return errors.Wrapf(err, "failed to make AudioBoardClearRoutes xmlrpc request with args busNumber: %v", busNumber)
	}
	return nil

}

// Reset resets Chameleon board
func (ch *Chameleon) Reset(ctx context.Context) error {
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("Reset")); err != nil {
		return errors.Wrap(err, "failed to make Reset xmlrpc request")
	}
	return nil
}
