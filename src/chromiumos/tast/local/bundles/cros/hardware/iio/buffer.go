package iio

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
)

// Buffer provides a means to read continuous data captured from a sensor.
// See https://01.org/linuxgraphics/gfx-docs/drm/driver-api/iio/buffers.html#buffers.
type Buffer struct {
	Sensor     *Sensor
	Channels   []*ChannelSpec
	bufferFile *os.File
}

// ChannelSpec describes one piece of data that is read from an iio buffer.
type ChannelSpec struct {
	Index        int
	Name         string
	Signed       bool
	RealBits     uint
	StorageBits  uint
	Shift        uint
	Endianness   uint
	byteOrder    binary.ByteOrder
	storageBytes int
}

// BufferData is a single reading of data from a sensor's buffer.
type BufferData struct {
	buffer *Buffer
	data   [][]byte
}

const (
	// BE specifies that channel data is big endian
	BE = iota
	// LE specifies that channel data is little endian
	LE
)

// NewBuffer reads the sysfs directory of a sensor and returns a Buffer associated
// with a that sensor.
func (s *Sensor) NewBuffer() (Buffer, error) {
	var ret Buffer

	// Make sure sensor has a buffer
	_, err := s.ReadAttr("buffer/enable")
	if err != nil {
		return ret, errors.Wrap(err, "sensor does not have a buffer")
	}

	// Get all channels
	chDir := path.Join(basePath, iioBasePath, s.Path, "scan_elements")
	files, err := ioutil.ReadDir(chDir)
	if err != nil {
		return ret, errors.Wrap(err, "error reading scan_elements")
	}

	cm := map[int]*ChannelSpec{}
	max := 0
	for _, file := range files {
		ch, err := s.parseChannel(file.Name())
		if err == nil {
			cm[ch.Index] = ch
			if ch.Index > max {
				max = ch.Index
			}
		}
	}

	ret.Channels = []*ChannelSpec{}
	for i := 0; i <= max; i++ {
		c, ok := cm[i]
		if !ok {
			return ret, errors.Errorf("missing channel index %v", i)
		}

		ret.Channels = append(ret.Channels, c)
	}

	ret.Sensor = s
	return ret, nil
}

// SetLength sets the maximun number of elements that can be buffered in the
// sensor's fifo.
func (b *Buffer) SetLength(len int) error {
	err := b.Sensor.WriteAttr("buffer/enable", "0")
	if err != nil {
		return errors.Wrap(err, "error disabling buffer")
	}

	err = b.Sensor.WriteAttr("buffer/length", strconv.Itoa(len))
	if err != nil {
		return errors.Wrap(err, "error setting buffer length")
	}

	return nil
}

// Open enables a sensor's buffer for reading and returns a channel to read
// data from the buffer.
func (b *Buffer) Open() (<-chan BufferData, error) {
	err := b.enableAllChannels()
	if err != nil {
		return nil, errors.Wrap(err, "error enabling channels")
	}

	err = b.Sensor.WriteAttr("buffer/enable", "1")
	if err != nil {
		return nil, errors.Wrap(err, "error enabling buffer")
	}

	f, err := os.Open(path.Join(basePath, "/dev", b.Sensor.Path))
	if err != nil {
		return nil, errors.Wrap(err, "error opening sensor buffer")
	}

	bits := 0
	for _, ch := range b.Channels {
		bits += int(ch.StorageBits)
	}

	buf := make([]byte, bits/8)
	events := make(chan BufferData)
	go func() {
		for {
			n, err := io.ReadFull(f, buf)
			if n == len(buf) {
				d, err := b.bufferData(buf)

				if err == nil {
					events <- d
				}
			}

			if err != nil {
				close(events)
				return
			}
		}
	}()

	b.bufferFile = f
	return events, nil
}

func (b *Buffer) enableAllChannels() error {
	for _, ch := range b.Channels {
		err := b.Sensor.WriteAttr("scan_elements/"+ch.Name+"_en", "1")
		if err != nil {
			return errors.Wrapf(err, "error enabling %v", ch.Name)
		}
	}

	return nil
}

func (b *Buffer) bufferData(buf []byte) (BufferData, error) {
	var ret BufferData

	cpy := make([]byte, len(buf))
	copy(cpy, buf)
	ret.buffer = b
	ret.data = make([][]byte, len(b.Channels))

	pos := 0
	for i := 0; i < len(b.Channels); i++ {
		c := b.Channels[i].storageBytes
		ret.data[i] = cpy[pos : pos+c]
		pos += c
	}
	return ret, nil
}

// Close closes an open buffer file and disables the buffer.
func (b *Buffer) Close() error {
	if b.bufferFile == nil {
		return errors.New("buffer is not open")
	}

	f := b.bufferFile
	b.bufferFile = nil

	err := f.Close()
	if err != nil {
		return errors.Wrap(err, "error closing buffer file")
	}

	err = b.Sensor.WriteAttr("buffer/enable", "0")
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

	re = regexp.MustCompile("^([lb]e):([su])(\\d+)/(\\d+)>>(\\d+)$")
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
	if ix >= len(d.buffer.Channels) {
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

// GetUint8 gets an uint8 from channel ix.
func (d *BufferData) GetUint8(ix int) (uint8, error) {
	c, err := d.getChannel(ix, 8, false)
	if err != nil {
		return 0, err
	}

	return uint8(d.data[ix][0]) >> c.Shift, nil
}

// GetInt8 gets an int8 from channel ix.
func (d *BufferData) GetInt8(ix int) (int8, error) {
	c, err := d.getChannel(ix, 8, true)
	if err != nil {
		return 0, err
	}

	return int8(d.data[ix][0]) >> c.Shift, nil
}

// GetUint16 gets an uint16 from channel ix.
func (d *BufferData) GetUint16(ix int) (uint16, error) {
	c, err := d.getChannel(ix, 16, false)
	if err != nil {
		return 0, err
	}

	return c.byteOrder.Uint16(d.data[ix]) >> c.Shift, nil
}

// GetInt16 gets an int16 from channel ix.
func (d *BufferData) GetInt16(ix int) (int16, error) {
	c, err := d.getChannel(ix, 16, true)
	if err != nil {
		return 0, err
	}

	return int16(c.byteOrder.Uint16(d.data[ix])) >> c.Shift, nil
}

// GetUint32 gets an uint32 from channel ix.
func (d *BufferData) GetUint32(ix int) (uint32, error) {
	c, err := d.getChannel(ix, 32, false)
	if err != nil {
		return 0, err
	}

	return c.byteOrder.Uint32(d.data[ix]) >> c.Shift, nil
}

// GetInt32 gets an int32 from channel ix.
func (d *BufferData) GetInt32(ix int) (int32, error) {
	c, err := d.getChannel(ix, 32, true)
	if err != nil {
		return 0, err
	}

	return int32(c.byteOrder.Uint32(d.data[ix])) >> c.Shift, nil
}

// GetUint64 gets an uint64 from channel ix.
func (d *BufferData) GetUint64(ix int) (uint64, error) {
	c, err := d.getChannel(ix, 64, false)
	if err != nil {
		return 0, err
	}

	return c.byteOrder.Uint64(d.data[ix]) >> c.Shift, nil
}

// GetInt64 gets an int64 from channel ix.
func (d *BufferData) GetInt64(ix int) (int64, error) {
	c, err := d.getChannel(ix, 64, true)
	if err != nil {
		return 0, err
	}

	return int64(c.byteOrder.Uint64(d.data[ix])) >> c.Shift, nil
}

// GetRaw gets the raw bytes from channel ix.
func (d *BufferData) GetRaw(ix int) ([]byte, error) {
	if ix >= len(d.buffer.Channels) {
		return nil, errors.Errorf("invalid channel index %v, buffer has %v",
			ix, len(d.buffer.Channels))
	}

	return d.data[ix], nil
}
