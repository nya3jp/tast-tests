// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iio

import (
	"context"
	"encoding/binary"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestRing(t *testing.T) {
	defer setupTestFiles(t, map[string]string{
		"iio:device0/name":                                "cros-ec-ring",
		"iio:device0/buffer/enable":                       "0",
		"iio:device0/buffer/length":                       "2",
		"iio:device0/scan_elements/in_accel_flag_en":      "0",
		"iio:device0/scan_elements/in_accel_flag_index":   "1",
		"iio:device0/scan_elements/in_accel_flag_type":    "le:u8/8>>0",
		"iio:device0/scan_elements/in_accel_id_en":        "0",
		"iio:device0/scan_elements/in_accel_id_index":     "0",
		"iio:device0/scan_elements/in_accel_id_type":      "le:u8/8>>0",
		"iio:device0/scan_elements/in_accel_x_ring_en":    "0",
		"iio:device0/scan_elements/in_accel_x_ring_index": "2",
		"iio:device0/scan_elements/in_accel_x_ring_type":  "le:s16/16>>0",
		"iio:device0/scan_elements/in_accel_y_ring_en":    "0",
		"iio:device0/scan_elements/in_accel_y_ring_index": "3",
		"iio:device0/scan_elements/in_accel_y_ring_type":  "le:s16/16>>0",
		"iio:device0/scan_elements/in_accel_z_ring_en":    "0",
		"iio:device0/scan_elements/in_accel_z_ring_index": "4",
		"iio:device0/scan_elements/in_accel_z_ring_type":  "le:s16/16>>0",
		"iio:device0/scan_elements/in_timestamp_en":       "0",
		"iio:device0/scan_elements/in_timestamp_index":    "5",
		"iio:device0/scan_elements/in_timestamp_type":     "le:s64/64>>0",
		"iio:device1/name":                                "cros-ec-accel",
		"iio:device1/location":                            "lid",
		"iio:device1/id":                                  "0",
		"iio:device1/scale":                               "0.25",
		"iio:device1/frequency":                           "100",
		"iio:device1/min_frequency":                       "100",
		"iio:device1/max_frequency":                       "1000",
		"iio:device2/name":                                "cros-ec-gyro",
		"iio:device2/location":                            "base",
		"iio:device2/id":                                  "1",
		"iio:device2/scale":                               "0.01",
		"iio:device2/sampling_frequency_available":        "0.000000 2000.000000 5000.000000",
		"iio:device2/buffer/hwfifo_timeout":               "0",
	})()

	ringData := []SensorReading{
		// Sensor flush events
		{[]float64{0, 0, 0}, 0, 0x3, 100},
		{[]float64{0, 0, 0}, 1, 0x3, 200},
		// Data events
		{[]float64{.25, .5, 1}, 0, 0x0, 300},
		{[]float64{.01, .02, .1}, 1, 0x0, 400},
		{[]float64{1, 2, 3}, 1, 0x0, 500},
		{[]float64{-4, -5, 6}, 1, 0x0, 600},
		{[]float64{0, -.25, 1.75}, 0, 0x0, 700},
		{[]float64{40, -50, 60}, 1, 0x0, 800},
	}

	// The first 2 events are flush events
	flushEvents := 2

	dutSensors, err := GetSensors()
	if err != nil {
		t.Fatal("Error reading sensors on DUT: ", err)
	}

	ring, err := NewRing(dutSensors)
	if err != nil {
		t.Fatal("Sensor ring not found: ", err)
	}

	if err := os.MkdirAll(path.Join(basePath, "dev"), 0755); err != nil {
		t.Fatal("Error making dev dir: ", err)
	}

	fifoFile := path.Join(basePath, "dev/iio:device0")

	// Use mkfifo to simulate an iio buffer
	if err := unix.Mkfifo(fifoFile, 0600); err != nil {
		t.Fatal("Error making buffer fifo: ", err)
	}

	go func() {
		bytes := make([]byte, 16)

		f, err := os.OpenFile(fifoFile, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal("Error opening named pipe for writing: ", err)
		}
		defer f.Close()

		for _, r := range ringData {
			s := ring.Sensors[r.ID]
			bytes[0] = uint8(r.ID)
			bytes[1] = uint8(r.Flags)
			binary.LittleEndian.PutUint16(bytes[2:4], uint16(r.Data[0]/s.Sensor.Scale))
			binary.LittleEndian.PutUint16(bytes[4:6], uint16(r.Data[1]/s.Sensor.Scale))
			binary.LittleEndian.PutUint16(bytes[6:8], uint16(r.Data[2]/s.Sensor.Scale))
			binary.LittleEndian.PutUint64(bytes[8:], uint64(r.Timestamp))

			if _, err = f.Write(bytes); err != nil {
				t.Fatalf("Error writing %v to named pipe: %v", bytes, err)
			}
		}
	}()

	readings, err := ring.Open(context.Background())
	if err != nil {
		t.Fatal("Error opening ring: ", err)
	}
	defer ring.Close()

	var read []SensorReading
	timeout := time.After(5 * time.Second)
l:
	for {
		select {
		case r, ok := <-readings:
			if !ok {
				t.Fatal("Ring unexpectedly closed")
			}
			read = append(read, *r)
			if len(read) == len(ringData)-flushEvents {
				break l
			}
		case <-timeout:
			t.Fatal("Timeout reading from ring")
		}
	}

	if !reflect.DeepEqual(ringData[flushEvents:], read) {
		t.Fatalf("Error reading from ring buffer: got %v; want %v",
			read, ringData[flushEvents:])
	}
}
