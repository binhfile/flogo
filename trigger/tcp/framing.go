package tcp

import (
	"fmt"
)

const (
	frmStateReceiveHeader = iota + 1
	frmStateReceiveData
)

type Frame struct {
	FrameMaxSize   int
	HeaderSize     int
	GetPayloadSize func(header []byte) int

	state        int
	header       []byte
	data         []byte
	dataLength   int
	recvHdrSize  int
	recvDataSize int
}

func (f *Frame) Initialize() error {
	if f.FrameMaxSize == 0 {
		return fmt.Errorf("frame max size is not set")
	}
	if f.FrameMaxSize > 1024*1024*1024 {
		return fmt.Errorf("frame max size is too big %d > %d", f.FrameMaxSize, 1024*1024*1024)
	}
	if f.HeaderSize == 0 {
		return fmt.Errorf("header size is not set")
	}
	if f.HeaderSize > 1024*1024*1024 {
		return fmt.Errorf("header size is too big %d > %d", f.HeaderSize, 1024*1024*1024)
	}
	if f.GetPayloadSize == nil {
		return fmt.Errorf("get payload size callback is not set")
	}

	f.data = make([]byte, f.FrameMaxSize)
	f.header = make([]byte, f.HeaderSize) // Fixed to 4 bytes prefix length in big - endian format
	f.Reset()
	return nil
}
func (f *Frame) Destroy() {
}
func (f *Frame) Reset() {
	f.state = frmStateReceiveHeader
	f.dataLength = 0
	f.recvHdrSize = 0
}
func (f *Frame) ByteToFrame(data []byte, callback func(header []byte, payload []byte)) error {
	for len(data) > 0 {
		if f.state == frmStateReceiveHeader {
			copiedSize := copy(f.header[f.recvHdrSize:], data)
			data = data[copiedSize:]
			f.recvHdrSize += copiedSize
			if f.recvHdrSize >= f.HeaderSize {
				f.dataLength = f.GetPayloadSize(f.header)
				if f.dataLength == 0 {
					return fmt.Errorf("get payload size callback return 0")
				}
				if f.dataLength > f.FrameMaxSize {
					return fmt.Errorf("get payload size callback return %d > frame max size %d",
						f.dataLength, f.FrameMaxSize)
				}
				f.state = frmStateReceiveData
				f.recvDataSize = 0
			}
			f.recvHdrSize = 0
		} else {
			copiedSize := copy(f.data[f.recvDataSize:f.dataLength], data)
			data = data[copiedSize:]
			f.recvDataSize += copiedSize
			if f.recvDataSize >= f.dataLength {
				callback(f.header, f.data[:f.dataLength])
				f.recvDataSize = 0
				f.state = frmStateReceiveHeader
			}
		}
	}
	return nil
}
func (f *Frame) FrameToBytes(frame []byte, BuildHeaderCallback func(header []byte, payloadSize int)) []byte {
	bytes := make([]byte, len(frame)+f.HeaderSize)
	BuildHeaderCallback(bytes[:f.HeaderSize], len(frame))
	copy(bytes[f.HeaderSize:], frame)
	return bytes
}
