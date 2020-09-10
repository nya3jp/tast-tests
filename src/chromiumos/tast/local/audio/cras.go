// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio interacts with audio operation.
package audio

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	crasPath      = "/usr/bin/cras"
	dbusName      = "org.chromium.cras"
	dbusPath      = "/org/chromium/cras"
	dbusInterface = "org.chromium.cras.Control"
)

// StreamType is used to specify the type of node we want to use for tests and
// helper functions.
type StreamType uint

const (
	// InputStream describes nodes with true IsInput attributes.
	InputStream StreamType = 1 << iota
	// OutputStream describes nodes with false IsInput attributes.
	OutputStream
)

func (t StreamType) String() string {
	switch t {
	case InputStream:
		return "InputStream"
	case OutputStream:
		return "OutputStream"
	default:
		return fmt.Sprintf("StreamType(%#x)", t)
	}
}

// Cras is used to interact with the cras process over D-Bus.
// For detailed spec, please find src/third_party/adhd/cras/README.dbus-api.
type Cras struct {
	obj dbus.BusObject
}

// NewCras connects to CRAS via D-Bus and returns a Cras object.
func NewCras(ctx context.Context) (*Cras, error) {
	testing.ContextLogf(ctx, "Waiting for %s D-Bus service", dbusName)
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Cras{obj}, nil
}

// CrasNode contains the metadata of Node in Cras.
// Currently fields which are actually needed by tests are defined.
// Please find src/third_party/adhd/cras/README.dbus-api for the meaning of
// each fields.
type CrasNode struct {
	ID         uint64
	Type       string
	Active     bool
	IsInput    bool
	DeviceName string
	NodeVolume uint64
}

// GetNodes calls cras.Control.GetNodes over D-Bus.
func (c *Cras) GetNodes(ctx context.Context) ([]CrasNode, error) {
	call := c.call(ctx, "GetNodes")
	if call.Err != nil {
		return nil, call.Err
	}

	// cras.Control.GetNodes D-Bus method's signature is not fixed.
	// Specifically, the number of output values depends on the actual
	// number of nodes.
	// That usage is not common practice, and it is less supported in
	// godbus. Here, instead, values are manually converted via
	// dbus.Variant.
	nodes := make([]CrasNode, len(call.Body))
	for i, n := range call.Body {
		mp := n.(map[string]dbus.Variant)
		if id, ok := mp["Id"]; !ok {
			return nil, errors.Errorf("'Id' not found: %v", mp)
		} else if nodes[i].ID, ok = id.Value().(uint64); !ok {
			return nil, errors.Errorf("'Id' is not uint64: %v", mp)
		}
		if nodeType, ok := mp["Type"]; !ok {
			return nil, errors.Errorf("'Type' not found: %v", mp)
		} else if nodes[i].Type, ok = nodeType.Value().(string); !ok {
			return nil, errors.Errorf("'Type' is not string: %v", mp)
		}
		if active, ok := mp["Active"]; !ok {
			return nil, errors.Errorf("'Active' not found: %v", mp)
		} else if nodes[i].Active, ok = active.Value().(bool); !ok {
			return nil, errors.Errorf("'Active' is not bool: %v", mp)
		}
		if isInput, ok := mp["IsInput"]; !ok {
			return nil, errors.Errorf("'IsInput' not found: %v", mp)
		} else if nodes[i].IsInput, ok = isInput.Value().(bool); !ok {
			return nil, errors.Errorf("'IsInput' is not bool: %v", mp)
		}
		if deviceName, ok := mp["DeviceName"]; !ok {
			return nil, errors.Errorf("'DeviceName' not found: %v", mp)
		} else if nodes[i].DeviceName, ok = deviceName.Value().(string); !ok {
			return nil, errors.Errorf("'DeviceName' is not string: %v", mp)
		}
		if nodeVolume, ok := mp["NodeVolume"]; !ok {
			return nil, errors.Errorf("'NodeVolume' not found: %v", mp)
		} else if nodes[i].NodeVolume, ok = nodeVolume.Value().(uint64); !ok {
			return nil, errors.Errorf("'NodeVolume' is not uint64: %v", mp)
		}
	}
	return nodes, nil
}

// GetNodeByType returns the first node with given type.
func (c *Cras) GetNodeByType(ctx context.Context, t string) (*CrasNode, error) {
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		if n.Type == t {
			return &n, nil
		}
		// Regard the front mic as the internal mic.
		if t == "INTERNAL_MIC" && n.Type == "FRONT_MIC" {
			return &n, nil
		}
	}

	return nil, errors.Errorf("failed to find a node with type %s", t)
}

// call is a wrapper around CallWithContext for convenience.
func (c *Cras) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// SetActiveNode calls cras.Control.SetActiveInput(Output)Node over D-Bus.
func (c *Cras) SetActiveNode(ctx context.Context, node CrasNode) error {
	cmd := "SetActiveOutputNode"
	if node.IsInput {
		cmd = "SetActiveInputNode"
	}
	return c.call(ctx, cmd, node.ID).Err
}

// SetActiveNodeByType sets node with specified type active.
func (c *Cras) SetActiveNodeByType(ctx context.Context, nodeType string) error {
	var node *CrasNode

	// Wait until the node with this type is existing.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := c.GetNodeByType(ctx, nodeType)
		node = n
		return err
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		return errors.Errorf("failed to wait node %s", nodeType)
	}

	if err := c.SetActiveNode(ctx, *node); err != nil {
		return errors.Errorf("failed to set node %s active", nodeType)
	}

	// Wait until that node is active.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := c.GetNodeByType(ctx, nodeType)
		if err != nil {
			return err
		}
		if !n.Active {
			return errors.New("node is not active")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		return errors.Errorf("failed to wait node %s to be active", nodeType)
	}

	return nil
}

// SetOutputNodeVolume calls cras.Control.SetOutputNodeVolume over D-Bus.
func (c *Cras) SetOutputNodeVolume(ctx context.Context, node CrasNode, volume int) error {
	return c.call(ctx, "SetOutputNodeVolume", node.ID, volume).Err
}

// WaitForDevice waits for specified types of stream nodes to be active.
// You can pass the streamType as a bitmap to wait for both input and output
// nodes to be active. Ex: WaitForDevice(ctx, InputStream|OutputStream)
// It should be used to verify the target types of nodes exist and are
// active before the real test starts.
// Notice that some devices use their displays as an internal speaker
// (e.g. monroe). When a display is closed, the internal speaker is removed,
// too. For this case, we should call power.TurnOnDisplay to turn on a display
// to re-enable an internal speaker.
func WaitForDevice(ctx context.Context, streamType StreamType) error {
	checkActiveNodes := func(ctx context.Context) error {
		cras, err := NewCras(ctx)
		if err != nil {
			return err
		}
		crasNodes, err := cras.GetNodes(ctx)
		if err != nil {
			return err
		}

		var active StreamType
		for _, n := range crasNodes {
			if n.Active {
				if n.IsInput {
					active |= InputStream
				} else {
					active |= OutputStream
				}

				if streamType&active == streamType {
					return nil
				}
			}
		}

		return errors.Errorf("node(s) %+v not in requested state", crasNodes)
	}

	return testing.Poll(ctx, checkActiveNodes, &testing.PollOptions{Timeout: 10 * time.Second})
}

// GetCRASPID finds the PID of cras.
func GetCRASPID() (int, error) {
	all, err := process.Pids()

	if err != nil {
		return -1, err
	}

	for _, pid := range all {
		proc, err := process.NewProcess(pid)
		if err != nil {
			// Assume that the process exited.
			continue
		}

		exe, err := proc.Exe()
		if err != nil {
			continue
		}

		if exe == crasPath {
			return int(pid), nil
		}
	}
	return -1, errors.Errorf("%v process not found", crasPath)
}

// StreamInfo holds attributes of an active stream.
// It contains only test needed fields.
type StreamInfo struct {
	Direction   string
	Effects     uint64
	FrameRate   uint32
	NumChannels uint8
}

var streamInfoRegex = regexp.MustCompile("(.*):(.*)")

func newStreamInfo(s string) (*StreamInfo, error) {
	data := streamInfoRegex.FindAllStringSubmatch(s, -1)
	res := make(map[string]string)
	for _, kv := range data {
		k := kv[1]
		v := strings.Trim(kv[2], " ")
		res[k] = v
	}

	const (
		Direction   = "direction"
		Effects     = "effects"
		FrameRate   = "frame_rate"
		NumChannels = "num_channels"
	)

	// Checks all key exists.
	for _, k := range []string{Direction, Effects, FrameRate, NumChannels} {
		if _, ok := res[k]; !ok {
			return nil, errors.Errorf("missing key: %s in StreamInfo", k)
		}
	}

	effects, err := strconv.ParseUint(res[Effects], 0, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse StreamInfo::%s (value: %s)", Effects, res[Effects])
	}

	frameRate, err := strconv.ParseUint(res[FrameRate], 10, 32)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse StreamInfo::%s (value: %s)", FrameRate, res[FrameRate])
	}
	numChannels, err := strconv.ParseUint(res[NumChannels], 10, 8)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse StreamInfo::%s (value: %s)", NumChannels, res[NumChannels])
	}

	return &StreamInfo{
		Direction:   res[Direction],
		Effects:     effects,
		FrameRate:   uint32(frameRate),
		NumChannels: uint8(numChannels),
	}, nil
}

// PollStreamResult is the CRAS stream polling result.
type PollStreamResult struct {
	Streams []StreamInfo
	Error   error
}

// StartPollStreamWorker starts a goroutine to poll an active stream.
func StartPollStreamWorker(ctx context.Context, timeout time.Duration) <-chan PollStreamResult {
	resCh := make(chan PollStreamResult, 1)
	go func() {
		streams, err := WaitForStreams(ctx, timeout)
		resCh <- PollStreamResult{Streams: streams, Error: err}
	}()
	return resCh
}

// WaitForStreams returns error if it fails to detect any active streams.
func WaitForStreams(ctx context.Context, timeout time.Duration) ([]StreamInfo, error) {
	var streams []StreamInfo

	err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Polling active stream")
		var err error
		streams, err = dumpActiveStreams(ctx)
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}
		if len(streams) == 0 {
			return &noStreamError{E: errors.New("no stream detected")}
		}
		// There is some active streams.
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	return streams, err
}

// WaitForNoStream returns error if it fails to wait for all active streams to stop.
func WaitForNoStream(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Wait until there is no active stream")
		streams, err := dumpActiveStreams(ctx)
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}
		if len(streams) > 0 {
			return errors.New("active stream detected")
		}
		// No active stream.
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

type noStreamError struct {
	*errors.E
}

// dumpActiveStreams parses active streams from "cras_test_client --dump_audio_thread" log.
// The log format is defined in cras_test_client.c.
// The active streams section begins with: "-------------stream_dump------------" and ends with: "Audio Thread Event Log:"
// Each stream is separated by "\n\n"
// An example of "cras_test_client --dump_audio_thread" log is shown as below:
// -------------stream_dump------------
// stream: 94437376 dev: 6
// direction: Output
// stream_type: CRAS_STREAM_TYPE_DEFAULT
// client_type: CRAS_CLIENT_TYPE_PCM
// buffer_frames: 2000
// cb_threshold: 1000
// effects: 0x0000
// frame_rate: 8000
// num_channels: 1
// longest_fetch_sec: 0.004927402
// num_overruns: 0
// is_pinned: 0
// pinned_dev_idx: 0
// num_missed_cb: 0
// volume: 1.000000
// runtime: 26.168175600
// channel map:0 -1 -1 -1 -1 -1 -1 -1 -1 -1 -1
//
// stream: 94437379 dev: 2
// ...
//
// Audio Thread Event Log:
//
func dumpActiveStreams(ctx context.Context) ([]StreamInfo, error) {
	dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Errorf("failed to dump audio thread: %s", err)
	}

	streamSection := strings.Split(string(dump), "-------------stream_dump------------")
	if len(streamSection) != 2 {
		return nil, errors.New("failed to split log by stream_dump")
	}
	streamSection = strings.Split(streamSection[1], "Audio Thread Event Log:")
	if len(streamSection) == 1 {
		return nil, errors.New("invalid stream_dump")
	}
	str := strings.Trim(streamSection[0], " \n\t")

	// No active streams, return nil
	if str == "" {
		return nil, nil
	}

	var streams []StreamInfo
	for _, streamStr := range strings.Split(str, "\n\n") {
		stream, err := newStreamInfo(streamStr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse stream")
		}
		streams = append(streams, *stream)
	}
	return streams, nil
}
