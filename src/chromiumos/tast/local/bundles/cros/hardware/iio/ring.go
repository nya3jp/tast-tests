// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"strconv"
	"time"

	"chromiumos/tast/errors"
)

type SensorRing struct {
	Sensors map[uint]Sensor
	Ring    Buffer
}

// Ring buffer has 5 channels
const (
	sensorId = iota
	flags
	xAxis
	yAxis
	zAxis
	timestamp
)

func NewSensorRing(sensors []Sensor) (SensorRing, error) {
	var ret SensorRing

	ret.Sensors = map[uint]Sensor{}
	ringFound := false
	for _, s := range sensors {
		if s.Name == Ring {
			ring := s
			ret.Ring, _ = ring.NewBuffer()
			ringFound = true
		} else {
			ret.Sensors[s.ID] = s
		}
	}

	if !ringFound {
		return ret, errors.New("ring not found")
	}

	return ret, nil
}

func (sr *SensorRing) Open() (<-chan SensorReading, error) {
	err := sr.Ring.SetLength(4096)
	if err != nil {
		return nil, errors.Wrap(err, "error setting ring buffer length")
	}

	for ix, s := range sr.Sensors {
		err := sr.Collect(ix, 0, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "error setting frequency of %v %v to 0",
				s.Location, s.Name)
		}

		err = s.WriteAttr("flush", "1")
		if err != nil {
			return nil, errors.Wrapf(err, "error flushing %v", s.Path)
		}
	}

	data, err := sr.Ring.Open()
	if err != nil {
		return nil, errors.Wrap(err, "error opening ring buffer")
	}

	// Read the buffer until there are flush events for all sensors
	done := make(chan error)
	go func() {
		flush := map[uint]bool{}
		c := 0

		for id := range sr.Sensors {
			flush[id] = false
			c++
		}

		for d := range data {
			id, err := d.GetUint8(sensorId)
			if err != nil {
				done <- err
				return
			}

			flags, err := d.GetUint8(flags)
			if err != nil {
				done <- err
				return
			}

			if flags&0x1 != 0 && !flush[uint(id)] {
				flush[uint(id)] = true
				c--

				if c == 0 {
					done <- nil
					return
				}
			}

			if err != nil {
				done <- err
				return
			}
		}

		done <- errors.New("ring buffer closed unexpectedly")
	}()

	// Wait up to 5 seconds for flush to finish
	select {
	case err := <-done:
		if err != nil {
			sr.Ring.Close()
			return nil, errors.Wrap(err, "error flushing sensors")
		}
	case <-time.After(5 * time.Second):
		sr.Ring.Close()
		return nil, errors.New("timeout flushing sensors")
	}

	events := make(chan SensorReading)
	go func() {
		for d := range data {
			r, err := sr.parseEvent(&d)

			if err == nil {
				events <- r
			}

			if err != nil {
				close(events)
				return
			}
		}

		close(events)
	}()

	return events, nil
}

func (sr *SensorRing) Close() error {
	if err := sr.Ring.Close(); err != nil {
		return errors.Wrap(err, "error closing ring buffer")
	}
	return nil
}

func (sr *SensorRing) Collect(id uint, sensorFreq, interruptFreq int) error {
	s, _ := sr.Sensors[id]

	err := s.WriteAttr("frequency", strconv.Itoa(sensorFreq))
	if err != nil {
		return errors.Wrapf(err, "error setting frequency of %v", s.Path)
	}

	// sampling_frequency takes ms
	interruptPeriod := 0
	if interruptFreq > 0 {
		interruptPeriod = 1000000 / interruptFreq
	}

	err = s.WriteAttr("sampling_frequency", strconv.Itoa(interruptPeriod))
	if err != nil {
		return errors.Wrapf(err, "error setting sampling_frequency of %v", s.Path)
	}

	return nil
}

func (sr *SensorRing) parseEvent(b *BufferData) (SensorReading, error) {
	var r SensorReading

	id, err := b.GetUint8(sensorId)
	if err != nil {
		return r, errors.Wrap(err, "error getting sensor id")
	}

	s, ok := sr.Sensors[uint(id)]
	if !ok {
		return r, errors.Errorf("cannot find sensor with id %v", id)
	}
	r.ID = uint(id)

	flags, err := b.GetUint8(flags)
	if err != nil {
		return r, errors.Wrap(err, "error getting sensor flags")
	}
	r.Flags = flags

	x, err := b.GetInt16(xAxis)
	if err != nil {
		return r, errors.Wrap(err, "error getting x axis value")
	}

	y, err := b.GetInt16(yAxis)
	if err != nil {
		return r, errors.Wrap(err, "error getting y axis value")
	}

	z, err := b.GetInt16(zAxis)
	if err != nil {
		return r, errors.Wrap(err, "error getting z axis value")
	}

	r.Data = []float64{
		float64(x) * s.Scale,
		float64(y) * s.Scale,
		float64(z) * s.Scale,
	}

	time, err := b.GetInt64(timestamp)
	if err != nil {
		return r, errors.Wrap(err, "error getting timestamp value")
	}

	r.Timestamp = time

	return r, nil
}
