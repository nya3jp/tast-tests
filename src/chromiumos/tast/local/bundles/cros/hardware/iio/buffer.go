// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
)

// Buffer provides a means to read continuous data captured from a sensor.
// See https://01.org/linuxgraphics/gfx-docs/drm/driver-api/iio/buffers.html#buffers.
type Buffer struct {
	sensor     *Sensor
	Channels   []*ChannelSpec
	bufferFile *os.File
	events     chan BufferData
}

// ChannelSpec describes one piece of data that is read from an iio buffer.
type ChannelSpec struct {
	Index        int
	Name         string
	Signed       bool
	RealBits     uint
	StorageBits  uint
	Shift        uint
	Endianness   Endianness
	byteOrder    binary.ByteOrder
	storageBytes int
}

// BufferData is a single reading of data from a sensor's buffer.
type BufferData struct {
	buffer *Buffer
	data   [][]byte
}

// Endianness defines the endianness of a Buffer channel.
type Endianness int

const (
	// BE specifies that channel data is big endian.
	BE Endianness = iota
	// LE specifies that channel data is little endian.
	LE
)

// NewBuffer reads the sysfs directory of a sensor and returns a Buffer associated
// with a that sensor.
func (s *Sensor) NewBuffer() (*Buffer, error) {
	ret := &Buffer{sensor: s}

	// Make sure sensor has a buffer
	_, err := s.ReadAttr("buffer/enable")
	if err != nil {
		return nil, errors.Wrap(err, "sensor does not have a buffer")
	}

	// On 3.18, ring needs a trigger. It is usually set by Android, but it has
	// to be set if Android never ran.
	// On 4.4 and after, ring does not need a trigger, current_trigger does not exist.
	curr, err := s.ReadAttr("trigger/current_trigger")
	if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrap(err, "current_trigger exists but not readable")
	}
	if err == nil && curr == "" {
		triggers, err := GetTriggers()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to list triggers")
		}
		found := false
		for _, t := range triggers {
			if t.Type == RingTrigger {
				if err := s.WriteAttr("trigger/current_trigger", t.Name); err != nil {
					return nil, errors.Wrap(err, "Error updating trigger")
				}
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("Unable to set the trigger properly")
		}
	}

	// Get all channels
	chDir := filepath.Join(basePath, iioBasePath, s.Path, "scan_elements")
	files, err := ioutil.ReadDir(chDir)
	if err != nil {
		return nil, errors.Wrap(err, "error reading scan_elements")
	}

	cm := map[int]*ChannelSpec{}
	max := -1
	for _, file := range files {
		ch, err := s.parseChannel(file.Name())
		if err == nil {
			cm[ch.Index] = ch
			if ch.Index > max {
				max = ch.Index
			}
		}
	}

	if max < 0 {
		return nil, errors.New("no valid channels")
	}

	for i := 0; i <= max; i++ {
		c, ok := cm[i]
		if !ok {
			return nil, errors.Errorf("missing channel index %v", i)
		}

		ret.Channels = append(ret.Channels, c)
	}

	return ret, nil
}

// SetLength sets the maximun number of elements that can be buffered in the
// sensor's fifo.
func (b *Buffer) SetLength(len int) error {
	err := b.sensor.WriteAttr("buffer/enable", "0")
	if err != nil {
		return errors.Wrap(err, "error disabling buffer")
	}

	err = b.sensor.WriteAttr("buffer/length", strconv.Itoa(len))
	if err != nil {
		return errors.Wrap(err, "error setting buffer length")
	}

	return nil
}

// Open enables a sensor's buffer for reading and returns a channel to read
// data from the buffer.
func (b *Buffer) Open() (<-chan BufferData, error) {
	if err := b.sensor.WriteAttr("buffer/enable", "0"); err != nil {
		return nil, errors.Wrap(err, "error disabling buffer")
	}

	if err := b.enableAllChannels(); err != nil {
		return nil, errors.Wrap(err, "error enabling channels")
	}

	if err := b.sensor.WriteAttr("buffer/enable", "1"); err != nil {
		return nil, errors.Wrap(err, "error enabling buffer")
	}

	f, err := os.Open(filepath.Join(basePath, "dev", b.sensor.Path))
	if err != nil {
		return nil, errors.Wrap(err, "error opening sensor buffer")
	}

	// Call SetDeadline to ensure the file can be read while being closed without
	// causing a race condition; see https://godoc.org/os#File.Close
	var t time.Time
	if err = f.SetDeadline(t); err != nil {
		return nil, errors.Wrap(err, "error setting file deadline")
	}

	bits := 0
	for _, ch := range b.Channels {
		bits += int(ch.StorageBits)
	}

	events := make(chan BufferData)
	go func() {
		buf := make([]byte, bits/8)
		for {
			if _, err := io.ReadFull(f, buf); err != nil {
				close(events)
				return
			}
			events <- b.bufferData(buf)
		}
	}()

	b.events = events
	b.bufferFile = f
	return events, nil
}

func (b *Buffer) enableAllChannels() error {
	for _, ch := range b.Channels {
		err := b.sensor.WriteAttr("scan_elements/"+ch.Name+"_en", "1")
		if err != nil {
			return errors.Wrapf(err, "error enabling %v", ch.Name)
		}
	}

	return nil
}

func (b *Buffer) bufferData(buf []byte) BufferData {
	ret := BufferData{buffer: b, data: make([][]byte, len(b.Channels))}
	cpy := make([]byte, len(buf))
	copy(cpy, buf)

	pos := 0
	for i, ch := range b.Channels {
		ret.data[i] = cpy[pos : pos+ch.storageBytes]
		pos += ch.storageBytes
	}
	return ret
}

// Close closes an open buffer file and disables the buffer.
func (b *Buffer) Close() error {
	if b.bufferFile == nil {
		return errors.New("buffer is not open")
	}

	events := b.events
	f := b.bufferFile
	b.events = nil
	b.bufferFile = nil

	err := f.Close()
	if err != nil {
		return errors.Wrap(err, "error closing buffer file")
	}

	// Ensure that the channel gets closed
	for range events {
	}

	err = b.sensor.WriteAttr("buffer/enable", "0")
	if err != nil {
		return errors.Wrap(err, "error disabling buffer")
	}
	return nil
}

func (s *Sensor) parseChannel(fn string) (*ChannelSpec, error) {
	var ret ChannelSpec

	re := regexp.MustCompile(`(.*)_type$`)
	matches := re.FindStringSubmatch(fn)
	if matches == nil {
		return nil, errors.New("not channel type")
	}

	ret.Name = matches[1]

	t, err := s.ReadAttr("scan_elements/" + fn)
	if err != nil {
		return nil, errors.Wrap(err, "error reading channel type")
	}

	re = regexp.MustCompile(`^([lb]e):([su])(\d+)/(\d+)>>(\d+)$`)
	matches = re.FindStringSubmatch(t)

	if matches == nil {
		return nil, errors.Errorf("bad type decriptor %q", t)
	}

	if matches[1] == "be" {
		ret.Endianness = BE
		ret.byteOrder = binary.BigEndian
	} else {
		ret.Endianness = LE
		ret.byteOrder = binary.LittleEndian
	}

	if matches[2] == "s" {
		ret.Signed = true
	} else {
		ret.Signed = false
	}

	val, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing number %q", matches[3])
	}
	ret.RealBits = uint(val)

	val, err = strconv.Atoi(matches[4])
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing number %q", matches[4])
	}
	ret.StorageBits = uint(val)
	ret.storageBytes = val / 8

	val, err = strconv.Atoi(matches[5])
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing number %q", matches[5])
	}
	ret.Shift = uint(val)

	ix, err := s.ReadAttr("scan_elements/" + ret.Name + "_index")
	if err != nil {
		return nil, errors.Wrapf(err, "error reading channel %v index", ret.Name)
	}

	val, err = strconv.Atoi(ix)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing number %q", ix)
	}
	ret.Index = val

	return &ret, nil
}

func (d *BufferData) getChannel(ix int, bits uint, signed bool) (*ChannelSpec, error) {
	if ix >= len(d.buffer.Channels) || ix < 0 {
		return nil, errors.Errorf("invalid channel index %v, buffer has %v",
			ix, len(d.buffer.Channels))
	}

	c := d.buffer.Channels[ix]
	if c.StorageBits != bits {
		return nil, errors.Errorf("channel size is %v bits; expected %v",
			c.StorageBits, bits)
	}

	if c.Signed != signed {
		return nil, errors.Errorf("channel value signed is %v; expected %v", c.Signed, signed)
	}

	return c, nil
}

// Uint8 gets an uint8 from channel ix.
func (d *BufferData) Uint8(ix int) (uint8, error) {
	c, err := d.getChannel(ix, 8, false)
	if err != nil {
		return 0, err
	}

	return uint8(d.data[ix][0]) >> c.Shift, nil
}

// Int8 gets an int8 from channel ix.
func (d *BufferData) Int8(ix int) (int8, error) {
	c, err := d.getChannel(ix, 8, true)
	if err != nil {
		return 0, err
	}

	return int8(d.data[ix][0]) >> c.Shift, nil
}

// Uint16 gets an uint16 from channel ix.
func (d *BufferData) Uint16(ix int) (uint16, error) {
	c, err := d.getChannel(ix, 16, false)
	if err != nil {
		return 0, err
	}

	return c.byteOrder.Uint16(d.data[ix]) >> c.Shift, nil
}

// Int16 gets an int16 from channel ix.
func (d *BufferData) Int16(ix int) (int16, error) {
	c, err := d.getChannel(ix, 16, true)
	if err != nil {
		return 0, err
	}

	return int16(c.byteOrder.Uint16(d.data[ix])) >> c.Shift, nil
}

// Uint32 gets an uint32 from channel ix.
func (d *BufferData) Uint32(ix int) (uint32, error) {
	c, err := d.getChannel(ix, 32, false)
	if err != nil {
		return 0, err
	}

	return c.byteOrder.Uint32(d.data[ix]) >> c.Shift, nil
}

// Int32 gets an int32 from channel ix.
func (d *BufferData) Int32(ix int) (int32, error) {
	c, err := d.getChannel(ix, 32, true)
	if err != nil {
		return 0, err
	}

	return int32(c.byteOrder.Uint32(d.data[ix])) >> c.Shift, nil
}

// Uint64 gets an uint64 from channel ix.
func (d *BufferData) Uint64(ix int) (uint64, error) {
	c, err := d.getChannel(ix, 64, false)
	if err != nil {
		return 0, err
	}

	return c.byteOrder.Uint64(d.data[ix]) >> c.Shift, nil
}

// Int64 gets an int64 from channel ix.
func (d *BufferData) Int64(ix int) (int64, error) {
	c, err := d.getChannel(ix, 64, true)
	if err != nil {
		return 0, err
	}

	return int64(c.byteOrder.Uint64(d.data[ix])) >> c.Shift, nil
}

// GetRaw gets the raw bytes from channel ix.
func (d *BufferData) GetRaw(ix int) ([]byte, error) {
	if ix >= len(d.buffer.Channels) || ix < 0 {
		return nil, errors.Errorf("invalid channel index %v, buffer has %v",
			ix, len(d.buffer.Channels))
	}

	return d.data[ix], nil
}
