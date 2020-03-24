package uhid

import (
	"strconv"
	"strings"
	"time"
)

func parseInfo(data *DeviceData, line string) error {
	var err error
	var bus uint64
	var vendorID uint64
	var productID uint64
	nextWhiteSpace := strings.Index(line, " ")
	if bus, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 16); err != nil {
		return err
	}
	line = line[nextWhiteSpace+1:]
	nextWhiteSpace = strings.Index(line, " ")
	if vendorID, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 32); err != nil {
		return err
	}
	if productID, err = strconv.ParseUint(line[nextWhiteSpace+1:], 16, 32); err != nil {
		return err
	}
	data.Bus = uint16(bus)
	data.VendorID = uint32(vendorID)
	data.ProductID = uint32(productID)
	return nil
}

func parseDescriptor(data *DeviceData, line string) error {
	nextWhiteSpace := strings.Index(line, " ")
	size, err := strconv.ParseInt(line[:nextWhiteSpace], 10, 0)
	if err != nil {
		return err
	}
	for i := 0; i < int(size); i++ {
		line = line[nextWhiteSpace+1:]
		var n uint64
		if n, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 8); err != nil {
			return err
		}
		data.Descriptor[i] = byte(n)
		nextWhiteSpace = strings.Index(line, " ")
	}
	return nil
}

func parseTime(line string) (time.Duration, error) {
	var seconds uint64
	var microSeconds uint64
	var err error
	if seconds, err = strconv.ParseUint(line[:6], 10, 32); err != nil {
		return time.Until(time.Now()), err
	}
	if microSeconds, err = strconv.ParseUint(line[7:13], 10, 32); err != nil {
		return time.Until(time.Now()), err
	}
	return time.Duration(seconds)*time.Second + time.Duration(microSeconds)*time.Microsecond, nil
}

func parseData(line string) ([]byte, error) {
	nextWhiteSpace := strings.Index(line, " ")
	var size uint64
	var err error
	if size, err = strconv.ParseUint(line[:nextWhiteSpace], 10, 0); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	for i := 0; i < int(size); i++ {
		line = line[nextWhiteSpace+1:]
		var n uint64
		if n, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 8); err != nil {
			return nil, err
		}
		data[i] = byte(n)
		nextWhiteSpace = strings.Index(line, " ")
	}
	return data, nil
}
