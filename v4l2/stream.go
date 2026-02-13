package v4l2

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func init() {
	if unsafe.Sizeof(Buffer{}) != 88 {
		panic("v4l2.Buffer size mismatch, check struct layout")
	}
}

type RequestBuffers struct {
	Count    uint32
	Type     uint32
	Memory   uint32
	Reserved [2]uint32
}

type Timeval struct {
	Sec  int64
	Usec int64
}

type Timecode struct {
	Type     uint32
	Flags    uint32
	Seconds  uint8
	Minutes  uint8
	Hours    uint8
	UserBits [4]uint8
}

type Buffer struct {
	Index     uint32
	Type      uint32
	BytesUsed uint32
	Flags     uint32
	Field     uint32

	Timestamp Timeval
	Timecode  Timecode

	Sequence uint32
	Memory   uint32

	M [8]byte

	Length    uint32
	Reserved2 uint32
	Reserved  uint32
}

func (b *Buffer) Offset() uint32 {
	return *(*uint32)(unsafe.Pointer(&b.M[0]))
}

func (b *Buffer) SetOffset(off uint32) {
	*(*uint32)(unsafe.Pointer(&b.M[0])) = off
}

type MmapBuffer struct {
	Data   []byte
	Lentgh uint32
}

const (
	MemoryMmap = 1
)

func CaptureOneFrame(fd uintptr, bufCount uint32) ([]byte, error) {
	if err := EnumFormats(fd); err != nil {
		return nil, err
	}

	if err := setFormat(fd, 640, 480, PixFmtpGAA); err != nil {
		return nil, err
	}

	bufs, err := initMmap(fd, bufCount)
	if err != nil {
		return nil, err
	}

	_, _, err = getCurrentFormat(fd)
	if err != nil {
		return nil, err
	}

	defer func() {
		for _, b := range bufs {
			if b.Data != nil {
				_ = unix.Munmap(b.Data)
			}
		}
	}()

	if err := startStreaming(fd, bufs); err != nil {
		return nil, err
	}

	frame, err := dequeueOneFrame(fd, bufs)

	if errStop := stopStreaming(fd); errStop != nil && err == nil {
		err = errStop
	}

	if err != nil {
		return nil, err
	}

	return frame, nil
}

func initMmap(fd uintptr, bufCount uint32) ([]MmapBuffer, error) {
	// preReqbufCleanup(fd)

	var req RequestBuffers
	req.Count = bufCount
	req.Type = BufTypeVideoCapture
	req.Memory = MemoryMmap

	reqBufs := iowr(uintptr('V'), VIDIOC_REQBUFS, uintptr(unsafe.Sizeof(req)))
	if err := ioctl(fd, reqBufs, unsafe.Pointer(&req)); err != nil {
		return nil, fmt.Errorf("VIDIOC_REQBUFS failed: %v", err)
	}

	if req.Count < 2 {
		return nil, fmt.Errorf("insufficient buffer memory: requested %d, got %d", bufCount, req.Count)
	}

	bufs := make([]MmapBuffer, req.Count)

	for i := uint32(0); i < req.Count; i++ {
		var buf Buffer
		buf.Type = BufTypeVideoCapture
		buf.Memory = MemoryMmap
		buf.Index = i

		reqQueryBuf := iowr(uintptr('V'), VIDIOC_QUERYBUF, uintptr(unsafe.Sizeof(buf)))
		if err := ioctl(fd, reqQueryBuf, unsafe.Pointer(&buf)); err != nil {
			return nil, fmt.Errorf("VIDIOC_QUERYBUF index=%d failed: %v", i, err)
		}
		// fmt.Printf("[%d] length: %d, offset: %d\n", i, buf.Length, buf.Offset())

		length := buf.Length
		offset := buf.Offset()

		data, err := unix.Mmap(
			int(fd),
			int64(offset),
			int(length),
			unix.PROT_READ|unix.PROT_WRITE,
			unix.MAP_SHARED,
		)
		if err != nil {
			return nil, fmt.Errorf("mmap index=%d failed: %v", i, err)
		}

		bufs[i] = MmapBuffer{
			Data:   data,
			Lentgh: length,
		}

	}
	return bufs, nil
}

func startStreaming(fd uintptr, bufs []MmapBuffer) error {
	for i := range bufs {
		var buf Buffer
		buf.Type = BufTypeVideoCapture
		buf.Memory = MemoryMmap
		buf.Index = uint32(i)

		reqBuf := iowr(uintptr('V'), VIDIOC_QBUF, uintptr(unsafe.Sizeof(buf)))
		if err := ioctl(fd, reqBuf, unsafe.Pointer(&buf)); err != nil {
			return fmt.Errorf("VIDIOC_QBUF index=%d failed: %v", i, err)
		}
	}

	bufType := uint32(BufTypeVideoCapture)
	reqStreamOn := iow(uintptr('V'), VIDIOC_STREAMON, uintptr(unsafe.Sizeof(bufType)))
	if err := ioctl(fd, reqStreamOn, unsafe.Pointer(&bufType)); err != nil {
		return fmt.Errorf("VIDIOC_STREAMON failed: %v", err)
	}

	return nil
}

func stopStreaming(fd uintptr) error {
	bufType := uint32(BufTypeVideoCapture)
	reqStreamOff := iow(uintptr('V'), VIDIOC_STREAMOFF, uintptr(unsafe.Sizeof(bufType)))
	if err := ioctl(fd, reqStreamOff, unsafe.Pointer(&bufType)); err != nil {
		return fmt.Errorf("VIDIOC_STREAMOFF failed: %v", err)
	}
	return nil
}

func dequeueOneFrame(fd uintptr, bufs []MmapBuffer) ([]byte, error) {
	var buf Buffer
	buf.Type = BufTypeVideoCapture
	buf.Memory = MemoryMmap

	reqDQBuf := iowr(uintptr('V'), VIDIOC_DQBUF, uintptr(unsafe.Sizeof(buf)))
	if err := ioctl(fd, reqDQBuf, unsafe.Pointer(&buf)); err != nil {
		return nil, fmt.Errorf("VIDIOC_DQBUF failed: %v", err)
	}

	idx := buf.Index
	if int(idx) >= len(bufs) {
		return nil, fmt.Errorf("DQBUF returned invalid index %d (n=%d)", idx, len(bufs))
	}

	used := buf.BytesUsed
	if used == 0 || used > bufs[idx].Lentgh {
		return nil, fmt.Errorf("unexpected bytesused=%d (buffer length=%d)", used, bufs[idx].Lentgh)
	}

	src := bufs[idx].Data[:used]

	frame := make([]byte, used)
	copy(frame, src)

	return frame, nil
}
