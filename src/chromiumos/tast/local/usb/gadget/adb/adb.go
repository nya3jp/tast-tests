package adb

import (
	"context"
	"fmt"
	"path/filepath"
	"os"
	"bytes"
	"syscall"
	"sync"
	"encoding/binary"

	"chromiumos/tast/local/usb/gadget"
	"chromiumos/tast/testing"
)

var (
	adbDesc = []byte {
		0x03, 0x00, 0x00, 0x00, 0x69, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00,
		0x03, 0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x09, 0x04, 0x00, 0x00, 0x02, 0xff, 0x42, 0x01,
		0x01, 0x07, 0x05, 0x81, 0x02, 0x00, 0x00, 0x00, 0x07, 0x05, 0x02, 0x02, 0x00, 0x00, 0x00, 0x09,
		0x04, 0x00, 0x00, 0x02, 0xff, 0x42, 0x01, 0x01, 0x07, 0x05, 0x81, 0x02, 0x00, 0x02, 0x00, 0x07,
		0x05, 0x02, 0x02, 0x00, 0x02, 0x01, 0x09, 0x04, 0x00, 0x00, 0x02, 0xff, 0x42, 0x01, 0x01, 0x07,
		0x05, 0x81, 0x02, 0x00, 0x04, 0x00, 0x06, 0x30, 0x00, 0x00, 0x00, 0x00, 0x07, 0x05, 0x02, 0x02,
		0x00, 0x04, 0x01, 0x06, 0x30, 0x00, 0x00, 0x00, 0x00 }
	adbStrings = []byte {
		0x02, 0x00, 0x00, 0x00, 0x1e, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x09, 0x04, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x53, 0x69, 0x6e, 0x6b, 0x00 }
)

type Adb struct {
	instance   string
	mount      string
	wait       sync.WaitGroup
	epCtrl    *os.File
	epIn      *os.File
	epOut     *os.File
	ctx        context.Context
	cancel     context.CancelFunc
}

const (
	A_SYNC = 0x434e5953
	A_CNXN = 0x4e584e43
	A_OPEN = 0x4e45504f
	A_OKAY = 0x59414b4f
	A_CLSE = 0x45534c43
	A_WRTE = 0x45545257
	A_AUTH = 0x48545541
	A_FFFF = 0xffffffff
)

type ucb struct {
	RequestType uint8
	Request     uint8
	Value       uint16
	Index       uint16
	Length      uint16
	EventType   uint8
	_           uint8
	_           uint16
}

type packet struct {
	Command uint32
	Arg1    uint32
	Arg2    uint32
	Length  uint32
	Crc32   uint32
	Magic   uint32
}

func crc32(d []byte) uint32 {
	var sum uint32
	sum = 0
	for _, v := range d {
		sum += uint32(v)
	}
	return sum
}

func NewAdb(instance string) *Adb {
	return &Adb{
		instance: instance,
		mount: fmt.Sprintf("/dev/ffs-adb-%s", instance),
	}
}

func (s *Adb) Name() string {
	return fmt.Sprintf("ffs.%s", s.instance)
}

func fcntl(fd uintptr, cmd uintptr, arg uintptr) (uintptr, error) {
	r, _, e := syscall.Syscall(syscall.SYS_FCNTL, fd, cmd, arg)
	if e != 0 {
		return 0, e
	}
	return r, nil
}

func (s *Adb) Start(ctx context.Context, _ gadget.ConfigFragment) (err error) {
	s.ctx, s.cancel = context.WithCancel(ctx)
	if err = syscall.Mkdir(s.mount, 0755); err != nil {
		return
	}
	if err = syscall.Mount(s.instance, s.mount, "functionfs", 0, ""); err != nil {
		syscall.Rmdir(s.mount)
		return
	}
	if s.epCtrl, err = os.OpenFile(filepath.Join(s.mount, "ep0"), os.O_RDWR | syscall.O_NONBLOCK, 0644); err != nil {
		syscall.Unmount(s.mount, syscall.MNT_FORCE)
		syscall.Rmdir(s.mount)
		return
	}
	if _, err = s.epCtrl.Write(adbDesc); err != nil {
		s.epCtrl.Close()
		syscall.Unmount(s.mount, syscall.MNT_FORCE)
		syscall.Rmdir(s.mount)
		return
	}
	if _, err = s.epCtrl.Write(adbStrings); err != nil {
		s.epCtrl.Close()
		syscall.Unmount(s.mount, syscall.MNT_FORCE)
		syscall.Rmdir(s.mount)
		return
	}
	if s.epIn, err = os.OpenFile(filepath.Join(s.mount, "ep1"), os.O_RDWR, 0644); err != nil {
	/* | syscall.O_NONBLOCK */
		s.epCtrl.Close()
		syscall.Unmount(s.mount, syscall.MNT_FORCE)
		syscall.Rmdir(s.mount)
		return
	}
	var f int
	if f, err = syscall.Open(filepath.Join(s.mount, "ep2"), os.O_RDWR, 0644); err != nil {
	/* | syscall.O_NONBLOCK */
		s.epIn.Close()
		s.epCtrl.Close()
		syscall.Unmount(s.mount, syscall.MNT_FORCE)
		syscall.Rmdir(s.mount)
		return
	} else {
		s.epOut = os.NewFile(uintptr(f), "epOut")
	}
	s.wait.Add(2)
	go s.processCtrl()
	go s.processOut()
	return nil
}

func (s *Adb) Stop() (err error) {
	s.cancel()
	// endpoint read might be in the interruptible wait, signal to wakeup from syscall.
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)
	if err := s.epCtrl.Close(); err != nil {
		testing.ContextLog(s.ctx, "epCtrl.Close(): ", err)
	}
	if err := s.epOut.Close(); err != nil {
		testing.ContextLog(s.ctx, "epOut.Close(): ", err)
	}
	if err := s.epIn.Close(); err != nil {
		testing.ContextLog(s.ctx, "epIn.Close(): ", err)
	}
	testing.ContextLog(s.ctx, "stop requested. Waiting.")
	s.wait.Wait()
	if err = syscall.Unmount(s.mount, syscall.MNT_FORCE); err != nil {
		return
	}
	if err = syscall.Rmdir(s.mount); err != nil {
		return
	}
	return nil
}

func (s *Adb) post(cmd, arg1, arg2 uint32, payload []byte) {
	msg := packet {
		Command : cmd,
		Arg1    : arg1,
		Arg2    : arg2,
		Magic   : cmd ^ A_FFFF,
	}
	testing.ContextLog(s.ctx, "post: ", msg, string(payload))
	if payload != nil {
		msg.Length = uint32(len(payload))
		msg.Crc32 = crc32(payload)
		binary.Write(s.epIn, binary.LittleEndian, &msg)
		binary.Write(s.epIn, binary.LittleEndian, &payload)
	} else {
		binary.Write(s.epIn, binary.LittleEndian, &msg)
	}
}

func (s *Adb) processCtrl() {
	var req ucb
	for {
		select {
		case <-s.ctx.Done():
			testing.ContextLog(s.ctx, "done: epCtrl")
			s.wait.Done()
			return
		default:
		}
		if err := binary.Read(s.epCtrl, binary.LittleEndian, &req); err != nil {
			testing.ContextLog(s.ctx, err)
			continue
		}
		testing.ContextLog(s.ctx, "epCtrl: ", req)
		if req.EventType != 4 {
			s.epCtrl.Write(nil)
			continue
		}
		if req.RequestType & 0x40 == 0 {
			continue
		} else if req.RequestType & 0x80 != 0 {
			if req.Length > 0 && req.Length < 4096 {
				s.epCtrl.Write(make([]byte, req.Length))
			} else {
				s.epCtrl.Write(nil)
			}
		} else {
			if req.Length > 0 && req.Length < 4096 {
				s.epCtrl.Read(make([]byte, req.Length))
			} else {
				s.epCtrl.Read(nil)
			}
		}
	}
}

func (s *Adb) processOut() {
	var req packet
	for {
		select {
		case <-s.ctx.Done():
			testing.ContextLog(s.ctx, "done: epOut")
			s.wait.Done()
			return
		default:
		}
		// TODO: does not work well.
		buf := make([]byte, 1024)
		if _, err := s.epOut.Read(buf); err != nil {
			testing.ContextLog(s.ctx, "epOut: ", err)
			continue
		}
//		if err := binary.Read(s.epOut, binary.LittleEndian, &req); err != nil {
		if err := binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &req); err != nil {
			testing.ContextLog(s.ctx, "epOut parse: ", err)
			continue
		}
		testing.ContextLog(s.ctx, "epOut: ", req)
		if req.Length > 0 && req.Length < 4096 {
			// consume payload, drop it.
			data := make([]byte, req.Length)
			s.epOut.Read(data)
			testing.ContextLog(s.ctx, "epOut: read more ", req.Length, string(data))
		}
		switch req.Command {
		case A_CNXN:
			s.post(A_CNXN, 0x01000000, 0x00100000, []byte("device::ro.product.model=DemoUSB;features=shell_v2"))
		case A_OPEN:
			s.post(A_CLSE, 0, 0, nil)
		}
	}
}
