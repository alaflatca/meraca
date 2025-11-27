package v4l2

import (
	"bytes"
	"fmt"
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
)

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

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i > 0 {
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

	reqQueryCap := ior(
		uintptr('V'),
		0,
		uintptr(unsafe.Sizeof(caps)),
	)
	fmt.Printf("VIDIOC_QUERYCAP req = 0x%x\n", reqQueryCap)

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
	if caps.Capabilities&caps.DeviceCaps != 0 {
		devCaps = caps.DeviceCaps
	}

	if devCaps&CapVideoCapture == 0 {
		return fmt.Errorf("device does not support VIDEO_CAPTURE")
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
	return nil
}
