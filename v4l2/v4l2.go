package v4l2

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// [ dir(2) ][ size(14) ][ type(8) ][ nr(8) ]
	//   ^가장 높은 비트                     ^가장 낮은 비트
	iocNRBits   = 8
	iocTypeBits = 8
	iocSizeBits = 14
	iocDirBits  = 2

	iocNRShift   = 0
	iocTypeShift = iocNRShift + iocNRBits     // 8
	iocSizeShift = iocTypeShift + iocTypeBits // 16
	iocDirShift  = iocSizeShift + iocSizeBits // 30

	iocNone  = 0 // 데이터 없음
	iocWrite = 1 // user -> kernel (보통 _IOW)
	iocRead  = 2 // kernel -> user (보통 _IOR)

	CapVideoCapture = 0x00000001
	CapStreaming    = 0x04000000
	CapDeviceCaps   = 0x80000000
	CAPReadWrite    = 0x00000002
)

type PixFormat struct {
	Width        uint32
	Height       uint32
	PixelFormat  uint32
	Field        uint32
	BytesPerLine uint32
	SizeImage    uint32
	Colorspace   uint32
	Priv         uint32
	Flags        uint32
	YcbcrEnc     uint32
	Quantization uint32
	XferFunc     uint32
}

type Format struct {
	Type uint32
	_    uint32
	fmt  [200]byte
}

func (f *Format) Pix() *PixFormat {
	return (*PixFormat)(unsafe.Pointer(&f.fmt[0]))
}

// 비트들 한 숫자로 합치기 (겹침 x)
func ioc(dir, typ, nr, size uintptr) uintptr {
	return (dir << iocDirShift) |
		(size << iocSizeShift) |
		(typ << iocTypeShift) |
		(nr << iocNRShift)
}

func ior(typ, nr, size uintptr) uintptr {
	return ioc(iocRead, typ, nr, size)
}
func iow(typ, nr, size uintptr) uintptr {
	return ioc(iocWrite, typ, nr, size)
}
func iowr(typ, nr, size uintptr) uintptr {
	return ioc(iocRead|iocWrite, typ, nr, size)
}

const (
	BufTypeVideoCapture = 1
)
const (
	FieldAny  = 0
	FieldNone = 1
)

func fourCC(a, b, c, d byte) uint32 {
	return uint32(a) |
		uint32(b)<<8 |
		uint32(c)<<16 |
		uint32(d)<<24
}

var (
	PixFmtYUYV  = fourCC('Y', 'U', 'Y', 'V')
	PixFmtMJPEG = fourCC('M', 'J', 'P', 'G')
)

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

type Capability struct {
	Driver       [16]byte
	Card         [32]byte
	BusInfo      [32]byte
	Version      uint32
	Capabilities uint32
	DeviceCaps   uint32
	Reserved     [3]uint32
}

func Start(fd uintptr) error {
	var caps Capability

	reqQueryCap := ior(uintptr('V'), VIDIOC_QUERYCAP, uintptr(unsafe.Sizeof(caps)))
	if err := ioctl(fd, reqQueryCap, unsafe.Pointer(&caps)); err != nil {
		return err
	}
	fmt.Println("==QUERYCAP==")
	fmt.Printf("%-16s : %q\n", "driver", cString(caps.Driver[:]))
	fmt.Printf("%-16s : %q\n", "card", cString(caps.Card[:]))
	fmt.Printf("%-16s : %q\n", "bus_info", cString(caps.BusInfo[:]))
	fmt.Printf("%-16s : %#x\n", "version", caps.Version)
	fmt.Printf("%-16s : %#x\n", "capabilities", caps.Capabilities)
	fmt.Printf("%-16s : %#x\n", "device_caps", caps.DeviceCaps)

	devCaps := caps.Capabilities
	// 이 디바이스는 device_caps를 쓰는 신형 스타일이다 라는 신호호
	if caps.Capabilities&CapDeviceCaps != 0 {
		devCaps = caps.DeviceCaps
	}

	if devCaps&CAPReadWrite == 0 {
		return fmt.Errorf("device does not support READ WRITE")
	} else {
		fmt.Println("=> device supports READ WRITE")
	}

	if devCaps&CapVideoCapture == 0 {
		return fmt.Errorf("device does not support VIDEO_CAPTURE")
	} else {
		fmt.Println("=> device supports VIDEO CAPTURE")
	}

	if devCaps&CapStreaming == 0 {
		fmt.Println("WARNING: device does not report STREAMING capability")
	} else {
		fmt.Println("=> device supports STREAMING")
	}

	if err := setFormatAndReadOneFrame(fd); err != nil {
		return err
	}

	return nil
}

func setFormatAndReadOneFrame(fd uintptr) error {
	var fmtV4L2 Format
	fmtV4L2.Type = BufTypeVideoCapture

	pix := fmtV4L2.Pix()

	pix.Width = 640
	pix.Height = 480
	// pix.PixelFormat = PixFmtMJPEG
	pix.PixelFormat = PixFmtYUYV
	pix.Field = FieldNone

	reqSetFmt := iowr(uintptr('V'), VIDIOC_S_FMT, uintptr(unsafe.Sizeof(fmtV4L2)))
	if err := ioctl(fd, reqSetFmt, unsafe.Pointer(&fmtV4L2)); err != nil {
		return fmt.Errorf("VIDIOC_S_FMT failed: %v", err)
	}
	fmt.Printf("VIDIOC_S_FMT req = %#x\n", reqSetFmt)

	fmt.Println("==S_FMT result==")
	fmt.Printf("size		: %d x %d\n", pix.Width, pix.Height)
	fmt.Printf("pixelformat	: %#x\n", pix.PixelFormat)
	fmt.Printf("bytesperline: %d\n", pix.BytesPerLine)
	fmt.Printf("sizeimage	: %d\n", pix.SizeImage)

	if pix.SizeImage == 0 {
		return fmt.Errorf("SizeImage == 0, cannot read freme size")
	}

	buf := make([]byte, pix.SizeImage)

	n, err := unix.Read(int(fd), buf)
	if err != nil {
		return fmt.Errorf("read frame failed: %v", err)
	}

	fmt.Printf("read() got %d bytes (expected ~%d)\n", n, pix.SizeImage)

	f, err := os.Create("one_frame")
	if err != nil {
		return fmt.Errorf("one_frame open failed: %v", err)
	}
	defer f.Close()
	if _, err = f.Write(buf[:n]); err != nil {
		return fmt.Errorf("one_frame write failed: %v", err)
	}

	return nil
}

func getCurrentFormat(fd uintptr) (*Format, *PixFormat, error) {
	var f Format
	f.Type = BufTypeVideoCapture

	reqGetFmt := iowr(uintptr('V'), VIDIOC_G_FMT, uintptr(unsafe.Sizeof(f)))
	if err := ioctl(fd, reqGetFmt, unsafe.Pointer(&f)); err != nil {
		return nil, nil, fmt.Errorf("VIDIOC_G_FMT failed: %v", err)
	}

	pix := f.Pix()
	log.Printf("G_FMT: %dx%d, pixelformat=0x%08x, sizeimage=%d, bytesperline=%d",
		pix.Width, pix.Height, pix.PixelFormat, pix.SizeImage, pix.BytesPerLine)

	return &f, pix, nil
}
