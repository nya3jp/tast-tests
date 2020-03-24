// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uhid

import (
  "bytes"
  "encoding/binary"
  "log"
  "os"

  "github.com/google/uuid"
)

const(
  HidMaxDescriptorSize = 4096

  UhidCreate2 uint32 = 0x0b
  UhidCreate2RequestSize = 4365
  UhidInput2 uint32 = 0x0c
)

type UhidCreate2Request struct {
  RequestType uint32
  Name [128]byte
  Phys [64]byte
  Uniq [64]byte
  DescriptorSize uint16
  Bus uint16
  VendorId uint32
  ProductId uint32
  Version uint32
  Country uint32
  Descriptor [HidMaxDescriptorSize]byte
}

type DeviceData struct {
  Name [128]byte
  Phys [64]byte
  Uniq [64]byte
  Descriptor [HidMaxDescriptorSize]byte
  Bus uint16
  VendorId uint32
  ProductId uint32
}

type UHIDDevice struct {
  Data DeviceData
  File *os.File
}

func (d *UHIDDevice) NewKernelDevice() {
  var err error
  d.File, err = os.OpenFile("/dev/uhid", os.O_RDWR, 0644)
  if err != nil {
    log.Fatal(err)
  }
  uniq, _ := uuid.NewRandom()
  copy(d.Data.Uniq[:], uniq[:])
  buf := new(bytes.Buffer)
  err = binary.Write(buf, binary.LittleEndian, d.Data.createRequest())
  if err != nil {
    log.Fatal(err)
  }
  _, err = d.File.Write(buf.Bytes())
  if err != nil {
    log.Fatal(err)
  }
}

func (d DeviceData) createRequest() UhidCreate2Request {
  r := UhidCreate2Request{}
  r.RequestType = UhidCreate2
  r.Name = d.Name
  r.Phys = d.Phys
  r.Uniq = d.Uniq
  r.DescriptorSize = uint16(len(d.Descriptor))
  r.Bus = d.Bus
  r.VendorId = d.VendorId
  r.ProductId = d.ProductId
  r.Version = 0
  r.Country = 0
  r.Descriptor = d.Descriptor
  return r
}
