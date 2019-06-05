package iio

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path"
	"reflect"
	"testing"
	"time"
)

func TestNewBuffer(t *testing.T) {
	defer setupTestFiles(t, map[string]string{
		"iio:device0/name":                     "cros-ec-accel",
		"iio:device0/location":                 "lid",
		"iio:device0/buffer/enable":            "0",
		"iio:device0/buffer/length":            "2",
		"iio:device0/scan_elements/el_a_en":    "0",
		"iio:device0/scan_elements/el_a_index": "0",
		"iio:device0/scan_elements/el_a_type":  "le:s8/8>>0",
		"iio:device0/scan_elements/el_b_en":    "0",
		"iio:device0/scan_elements/el_b_index": "1",
		"iio:device0/scan_elements/el_b_type":  "be:u14/16>>2",
		"iio:device0/scan_elements/el_c_en":    "0",
		"iio:device0/scan_elements/el_c_index": "3",
		"iio:device0/scan_elements/el_c_type":  "le:s29/32>>3",
		"iio:device0/scan_elements/el_d_en":    "0",
		"iio:device0/scan_elements/el_d_index": "2",
		"iio:device0/scan_elements/el_d_type":  "le:u64/64>>0",
	})()

	sensors, err := GetSensors()
	if err != nil {
		t.Fatal("Error getting sensors: ", err)
	}

	buffer, err := sensors[0].NewBuffer()
	if err != nil {
		t.Fatal("Error getting buffer: ", err)
	}

	expected := &Buffer{
		sensors[0], []*ChannelSpec{
			&ChannelSpec{0, "el_a", true, 8, 8, 0, LE, binary.LittleEndian, 1},
			&ChannelSpec{1, "el_b", false, 14, 16, 2, BE, binary.BigEndian, 2},
			&ChannelSpec{2, "el_d", false, 64, 64, 0, LE, binary.LittleEndian, 8},
			&ChannelSpec{3, "el_c", true, 29, 32, 3, LE, binary.LittleEndian, 4},
		}, nil, nil,
	}

	if !reflect.DeepEqual(expected, buffer) {
		t.Errorf("Unexpected buffer: got %v; want %v", buffer, expected)
	}
}

func TestOpenBuffer(t *testing.T) {
	defer setupTestFiles(t, map[string]string{
		"iio:device0/name":                     "cros-ec-accel",
		"iio:device0/location":                 "lid",
		"iio:device0/buffer/enable":            "0",
		"iio:device0/buffer/length":            "2",
		"iio:device0/scan_elements/el_a_en":    "0",
		"iio:device0/scan_elements/el_a_index": "0",
		"iio:device0/scan_elements/el_a_type":  "le:s16/16>>0",
		"iio:device0/scan_elements/el_b_en":    "0",
		"iio:device0/scan_elements/el_b_index": "1",
		"iio:device0/scan_elements/el_b_type":  "be:u30/32>>2",
	})()

	sensors, err := GetSensors()
	if err != nil {
		t.Fatal("Error getting sensors: ", err)
	}

	buffer, err := sensors[0].NewBuffer()
	if err != nil {
		t.Fatal("Error getting buffer: ", err)
	}

	if err := os.MkdirAll(path.Join(basePath, "dev"), 0755); err != nil {
		t.Fatal("Error making dev dir: ", err)
	}

	fifoFile := path.Join(basePath, "dev/iio:device0")

	// Use mkfifo to simulate an iio buffer
	cmd := exec.Command("mkfifo", fifoFile)
	if err := cmd.Run(); err != nil {
		t.Fatal("Error making buffer fifo: ", err)
	}

	go func() {
		var s16 int16
		var u32 uint32
		bytes := make([]byte, 6)
		sBytes := bytes[0:2]
		uBytes := bytes[2:6]

		f, err := os.OpenFile(fifoFile, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal("Error opening named pipe for writing: ", err)
		}
		defer f.Close()

		for i := 0; i < 5; i++ {
			s16 = -10 - int16(i)
			binary.LittleEndian.PutUint16(sBytes, uint16(s16))
			u32 = (20 + uint32(i)) << 2
			binary.BigEndian.PutUint32(uBytes, u32)

			_, err = f.Write(bytes)
			if err != nil {
				t.Fatalf("Error writing to named pipe %v: %v", i, err)
			}
		}
	}()

	recvData := []BufferData{}
	data, err := buffer.Open()
	if err != nil {
		t.Fatal("Error opening buffer: ", err)
	}
	defer buffer.Close()

	timeout := time.After(5 * time.Second)
l:
	for {
		select {
		case d, ok := <-data:
			recvData = append(recvData, d)

			if len(recvData) == 5 || !ok {
				break l
			}
		case <-timeout:
			t.Fatal("Timeout reading from buffer")
		}
	}

	if len(recvData) != 5 {
		t.Fatalf("Error reading buffer: got %v; want 5 elements", recvData)
	}

	for i := 0; i < 5; i++ {
		s, _ := recvData[i].Int16(0)
		if s != int16(-10-i) {
			t.Errorf("Wrong data value at index %v channel 0: got %v; want %v",
				i, s, -10-i)
		}

		u, _ := recvData[i].Uint32(1)
		if u != uint32(20+i) {
			t.Errorf("Wrong data value at index %v channel 1: got %v; want %v",
				i, u, 20+i)
		}
	}
}
