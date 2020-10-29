// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
)

// CrosRing is an instance of a cros-ec-ring iio device. The buffer for this
// device is used to receive sensor data for all cros-ec-* sensors on a DUT.
//
// Each datum read from the cros-ec-ring buffer contains a sensor ID field which
// coresponds to ID of the cros ec sensor which produced the data.
type CrosRing struct {
	// Sensors is a list of all of the cros ec sensors that will produce data
	// on the cros-ec-ring. The cros ec ring itself will not be present in this
	// list.
	Sensors map[uint]ringSensor
	// ring is the sensor buffer which belongs to the cros-ec-ring iio device
	ring *Buffer
	// events is a channel which produces sensor data read from the cros-ec-ring
	events chan *SensorReading
}

// ringSensor is a Sensor which belongs to a cros-ec-ring device, but not the
// cros-ec-ring device itself.
type ringSensor struct {
	Sensor *Sensor
}

// Ring buffer has 6 channels
const (
	// sensorID is the ID of sensor that this data belongs to. It is the
	// in_accel_id channel on the buffer.
	sensorID = 0
	// flags are flag values reported by the cros ec. It is the in_accel_flag
	// channel on the buffer.
	flags = 1
	// xAxis is the first data value from a sensor. It is the in_accel_x_ring
	// channel on the buffer.
	xAxis = 2
	// yAxis is the second data value from a sensor. It is the in_accel_y_ring
	// channel on the buffer.
	yAxis = 3
	// zAxis is the third data value from a sensor. It is the in_accel_z_ring
	// channel on the buffer.
	zAxis = 4
	// timestamp is the time since boot of the sensor reading in the ap's time
	// domain.  It is the in_timestamp channel on the buffer.
	timestamp = 5
	// The buffer has 6 total channels.
	ringChannels = 6
)

// NewRing creates a CrosRing from a list of Sensors on the DUT.
func NewRing(sensors []*Sensor) (*CrosRing, error) {
	ret := &CrosRing{Sensors: map[uint]ringSensor{}}

	for _, s := range sensors {
		if s.Name == Ring {
			if ret.ring != nil {
				return nil, errors.New("multiple cros-ec-ring iio devices found")
			}

			ring, err := s.NewBuffer()
			if err != nil {
				return nil, errors.Wrap(err, "error creating ring buffer")
			}

			if len(ring.Channels) != ringChannels {
				return nil, errors.Errorf("unexpected number of channels for ring buffer: got %v; want %v",
					len(ring.Channels), ringChannels)
			}
			ret.ring = ring
		} else {
			ret.Sensors[s.ID] = ringSensor{s}
		}
	}

	if ret.ring == nil {
		return nil, errors.New("ring not found")
	}

	return ret, nil
}

// Open will prepare the ring buffer to be read and open the buffer for reading.
// Once open it will flush all existing data from the sensors and return a channel
// of SensorReading which all new sensor readings will be published to.
func (cr *CrosRing) Open(ctx context.Context) (<-chan *SensorReading, error) {
	if err := cr.ring.SetLength(4096); err != nil {
		return nil, errors.Wrap(err, "error setting ring buffer length")
	}

	// Disable all sensors
	for _, s := range cr.Sensors {
		if err := s.Disable(); err != nil {
			return nil, errors.Wrapf(err, "failed to disable sensor %v %v",
				s.Sensor.Location, s.Sensor.Name)
		}
	}

	data, err := cr.ring.Open()
	if err != nil {
		return nil, errors.Wrap(err, "error opening ring buffer")
	}
	success := false
	defer func() {
		if !success {
			cr.ring.Close()
		}
	}()

	// Flush all sensors
	for _, s := range cr.Sensors {
		attr := "buffer/hwfifo_flush"

		if s.Sensor.OldSysfsStyle {
			attr = "flush"
		}
		if err := s.Sensor.WriteAttr(attr, "1"); err != nil {
			return nil, errors.Wrapf(err, "failed to flush %v", s.Sensor.Path)
		}
	}

	flush := map[uint]struct{}{}
	for id := range cr.Sensors {
		flush[id] = struct{}{}
	}

	// Read the buffer until there are flush events for all sensors.
	// Wait up to 5 seconds for flush to finish.
	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for len(flush) > 0 {
		select {
		case d, ok := <-data:
			if !ok {
				return nil, errors.New("ring buffer closed unexpectedly")
			}

			r, err := cr.parseEvent(&d)
			if err != nil {
				return nil, errors.Wrap(err, "error parsing event")
			}

			if r.Flags&flushFlag != 0 {
				delete(flush, r.ID)
			}
		case <-timeout.Done():
			return nil, errors.Wrap(timeout.Err(), "error flushing ring")
		}
	}

	events := make(chan *SensorReading)
	go func() {
		defer close(events)
		for d := range data {
			r, err := cr.parseEvent(&d)
			if err != nil {
				return
			}
			events <- r
		}
	}()
	cr.events = events

	success = true
	return events, nil
}

// Close will close an open ring buffer.
func (cr *CrosRing) Close() error {
	if err := cr.ring.Close(); err != nil {
		return errors.Wrap(err, "error closing ring buffer")
	}

	// Drain the events channel
	for range cr.events {
	}

	return nil
}

// Enable sets the sensor ODR and the interrupt frequency of a sensor. This should
// only be called after the ring buffer is open because all sensors will be disabled
// when the ring is opened.
func (s *ringSensor) Enable(sensorFreq, interruptFreq int) error {
	if s.Sensor.OldSysfsStyle {
		if err := s.Sensor.WriteAttr("frequency", strconv.Itoa(sensorFreq)); err != nil {
			return errors.Wrapf(err, "error setting frequency of %v %v to %v",
				s.Sensor.Location, s.Sensor.Name, sensorFreq)
		}
	} else {
		if err := s.Sensor.WriteAttr("sampling_frequency", fmt.Sprintf(
			"%d.%03d", sensorFreq/1000, sensorFreq%1000)); err != nil {
			return errors.Wrapf(err, "error setting frequency of %v %v to %v",
				s.Sensor.Location, s.Sensor.Name, sensorFreq)
		}
	}

	// sampling_frequency takes ms
	interruptPeriod := 0
	if interruptFreq > 0 {
		interruptPeriod = 1e6 / interruptFreq
	}

	if s.Sensor.OldSysfsStyle {
		if err := s.Sensor.WriteAttr("sampling_frequency", strconv.Itoa(interruptPeriod)); err != nil {
			return errors.Wrapf(err, "error setting sampling_frequency of %v %v to %v",
				s.Sensor.Location, s.Sensor.Name, interruptPeriod)
		}
	} else {
		if err := s.Sensor.WriteAttr("buffer/hwfifo_timeout", fmt.Sprintf(
			"%d.%03d", interruptPeriod/1000, interruptPeriod%1000)); err != nil {
			return errors.Wrapf(err, "error setting sampling_frequency of %v %v to %v",
				s.Sensor.Location, s.Sensor.Name, interruptPeriod)
		}
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

func (cr *CrosRing) parseEvent(b *BufferData) (*SensorReading, error) {
	var ret SensorReading

	id, err := b.Uint8(sensorID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting sensor id")
	}

	s, ok := cr.Sensors[uint(id)]
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

	t, err := b.Int64(timestamp)
	if err != nil {
		return nil, errors.Wrap(err, "error getting timestamp value")
	}
	ret.Timestamp = time.Duration(t)

	return &ret, nil
}

// BootTime returns the duration from the boot time of the DUT to now.
func BootTime() (time.Duration, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return 0, errors.Wrap(err, "error reading BootTime")
	}
	return time.Duration(ts.Nano()), nil
}

// Validate ensures the sample timestamps are within expected range.
func Validate(rs []*SensorReading, start, end time.Duration, sn *Sensor, collectTime time.Duration) error {
	var expected int
	if sn.Name == Light {
		// Light is on-change only. At worse, we may not see any sample if the light is very steady.
		expected = 0
	} else {
		// Expect that there are at least half the number of samples for the given frequency.
		expected = int(float64(sn.MaxFrequency)/1e3*collectTime.Seconds()) / 2
	}

	if len(rs) < expected {
		return errors.Errorf("not enough data collected for %v %v with %.2f Hz in %v: got %v; want at least %v",
			sn.Location, sn.Name, float64(sn.MaxFrequency)/1e3, collectTime, len(rs), expected)
	}

	last := start
	for ix, sr := range rs {
		if sr.Timestamp < last {
			return errors.Errorf("timestamp out of order for %v %v at index %v: got %v; want >= %v",
				sn.Location, sn.Name, ix, sr.Timestamp, last)
		}

		last = sr.Timestamp
		if sr.Timestamp > end {
			return errors.Errorf("timestamp in future for %v %v at index %v: got %v; want <= %v",
				sn.Location, sn.Name, ix, sr.Timestamp, end)
		}
	}
	return nil
}
