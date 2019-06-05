// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"strconv"
	"time"

	"chromiumos/tast/errors"
)

// CrosRing is an instance of a cros-ec-ring iio device. The buffer for this
// device is used to receive sensor data for all cros-ec-* sensors on a DUT.
type CrosRing struct {
	Sensors map[uint]ringSensor
	ring    *Buffer
	events  chan *SensorReading
}

type ringSensor struct {
	Sensor *Sensor
}

// Ring buffer has 6 channels
const (
	sensorID = iota
	flags
	xAxis
	yAxis
	zAxis
	timestamp
	ringChannels
)

// NewRing creates a CrosRing from a list of Sensors on the DUT.
func NewRing(sensors []*Sensor) (*CrosRing, error) {
	ret := &CrosRing{Sensors: map[uint]ringSensor{}}

	ringFound := false
	for _, s := range sensors {
		if s.Name == Ring {
			ring, err := s.NewBuffer()
			if err != nil {
				return nil, errors.Wrap(err, "error creating ring buffer")
			}

			if len(ring.Channels) != ringChannels {
				return nil, errors.Errorf("unexpected number of channels for ring buffer: got %v; want %v",
					len(ring.Channels), ringChannels)
			}
			ret.ring = ring
			ringFound = true
		} else {
			ret.Sensors[s.ID] = ringSensor{s}
		}
	}

	if !ringFound {
		return nil, errors.New("ring not found")
	}

	return ret, nil
}

// Open will prepare the ring buffer to be read and open the buffer for reading.
// Once open it will flush all existing data from the sensors and return a channel
// of SensorReading which all new sensor readings will be published to.
func (sr *CrosRing) Open() (<-chan *SensorReading, error) {
	err := sr.ring.SetLength(4096)
	if err != nil {
		return nil, errors.Wrap(err, "error setting ring buffer length")
	}

	for _, s := range sr.Sensors {
		err := s.Disable()
		if err != nil {
			return nil, errors.Wrapf(err, "error setting frequency of %v %v to 0",
				s.Sensor.Location, s.Sensor.Name)
		}

		err = s.Sensor.WriteAttr("flush", "1")
		if err != nil {
			return nil, errors.Wrapf(err, "error flushing %v", s.Sensor.Path)
		}
	}

	data, err := sr.ring.Open()
	if err != nil {
		return nil, errors.Wrap(err, "error opening ring buffer")
	}

	c := 0
	flush := map[uint]bool{}
	for id := range sr.Sensors {
		flush[id] = false
		c++
	}

	// Read the buffer until there are flush events for all sensors.
	// Wait up to 5 seconds for flush to finish.
	timeout := time.After(5 * time.Second)
	for c > 0 {
		select {
		case d, ok := <-data:
			if !ok {
				sr.ring.Close()
				return nil, errors.New("ring buffer closed unexpectedly")
			}

			r, err := sr.parseEvent(&d)
			if err != nil {
				sr.ring.Close()
				return nil, errors.Wrap(err, "error parsing event")
			}

			if r.Flags&0x1 != 0 && !flush[r.ID] {
				flush[r.ID] = true
				c--
			}
		case <-timeout:
			sr.ring.Close()
			return nil, errors.New("timeout flushing sensors")
		}
	}

	events := make(chan *SensorReading)
	go func() {
		defer close(events)
		for d := range data {
			r, err := sr.parseEvent(&d)
			if err != nil {
				return
			}
			events <- r
		}
	}()
	sr.events = events

	return events, nil
}

// Close will close an open ring buffer.
func (sr *CrosRing) Close() error {
	if err := sr.ring.Close(); err != nil {
		return errors.Wrap(err, "error closing ring buffer")
	}

	events := sr.events
	sr.events = nil
	for range events {
	}

	return nil
}

// Enable sets the sensor ODR and the interrupt frequency of a sensor. This should
// only be called after the ring buffer is open because all sensors will be disabled
// when the ring is opened.
func (s *ringSensor) Enable(sensorFreq, interruptFreq int) error {
	err := s.Sensor.WriteAttr("frequency", strconv.Itoa(sensorFreq))
	if err != nil {
		return errors.Wrapf(err, "error setting frequency of %v %v to %v",
			s.Sensor.Location, s.Sensor.Name, sensorFreq)
	}

	// sampling_frequency takes ms
	interruptPeriod := 0
	if interruptFreq > 0 {
		interruptPeriod = 1e6 / interruptFreq
	}

	err = s.Sensor.WriteAttr("sampling_frequency", strconv.Itoa(interruptPeriod))
	if err != nil {
		return errors.Wrapf(err, "error setting sampling_frequency of %v %v to %v",
			s.Sensor.Location, s.Sensor.Name, interruptPeriod)
	}

	return nil
}

// Disable will stop a sensor from colleting data by setting the ODR and interrupt
// frequency to zero.
func (s *ringSensor) Disable() error {
	if err := s.Enable(0, 0); err != nil {
		return errors.Wrap(err, "error disabling sensor")
	}
	return nil
}

func (sr *CrosRing) parseEvent(b *BufferData) (*SensorReading, error) {
	var ret SensorReading

	id, err := b.Uint8(sensorID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting sensor id")
	}

	s, ok := sr.Sensors[uint(id)]
	if !ok {
		return nil, errors.Errorf("cannot find sensor with id %v", id)
	}
	ret.ID = uint(id)

	flags, err := b.Uint8(flags)
	if err != nil {
		return nil, errors.Wrap(err, "error getting sensor flags")
	}
	ret.Flags = flags

	x, err := b.Int16(xAxis)
	if err != nil {
		return nil, errors.Wrap(err, "error getting x axis value")
	}

	y, err := b.Int16(yAxis)
	if err != nil {
		return nil, errors.Wrap(err, "error getting y axis value")
	}

	z, err := b.Int16(zAxis)
	if err != nil {
		return nil, errors.Wrap(err, "error getting z axis value")
	}

	ret.Data = []float64{
		float64(x) * s.Sensor.Scale,
		float64(y) * s.Sensor.Scale,
		float64(z) * s.Sensor.Scale,
	}

	time, err := b.Int64(timestamp)
	if err != nil {
		return nil, errors.Wrap(err, "error getting timestamp value")
	}
	ret.Timestamp = time

	return &ret, nil
}
