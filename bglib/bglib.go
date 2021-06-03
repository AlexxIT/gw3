package bglib

import (
	"encoding/hex"
	log "github.com/sirupsen/logrus"
	"io"
)

type Reader struct {
	r   io.Reader
	buf []byte
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r, make([]byte, 0, 1024)}
}

func (self *Reader) Read(p []byte) (int, error) {
	chunk := make([]byte, 1024)
	expectedLen := 0

	if len(self.buf) >= 3 && (self.buf[0] != 0xC0 || self.buf[1]&0x78 != 0x20 || self.buf[2] == 0) {
		log.Debugln("! Transparent proxy wrong buf", hex.EncodeToString(self.buf))
		n := copy(p, self.buf)
		self.buf = self.buf[:0]
		return n, nil
	}

	for {
		n, err := self.r.Read(chunk)
		if err != nil {
			return 0, err
		}

		// check start byte = 0xC0, type byte = 0xA0 or 0x20, len byte > 0
		if len(self.buf) == 0 && (chunk[0] != 0xC0 || chunk[1]&0x78 != 0x20 || chunk[2] == 0) {
			log.Debugln("! Transparent proxy wrong chunk", hex.EncodeToString(chunk[:n]))
			return copy(p, chunk[:n]), nil
		}

		self.buf = append(self.buf, chunk[:n]...)

		if expectedLen == 0 {
			// 0xC0 + cmdType + payloadLen + classID + commandID + payload + 0xC0
			expectedLen = 6 + int(self.buf[1]&0x07)<<16 + int(self.buf[2])
		}

		// check if loaded all data
		if len(self.buf) < expectedLen {
			continue
		}

		if self.buf[expectedLen-1] != 0xC0 && self.FixExtendedScanResponse() {
			//log.Debugln("FIX extended scan response", hex.EncodeToString(self.buf))
			if len(self.buf) < expectedLen {
				continue
			}
		}

		if self.buf[expectedLen-1] == 0xC0 {
			n = copy(p, self.buf[:expectedLen])
			// shift read buffer to next chunk
			self.buf = self.buf[expectedLen:]
			return n, nil
		} else {
			log.Debugln("! Wrong len or end byte", hex.EncodeToString(self.buf))
			expectedLen = 0
			self.buf = self.buf[:0]
		}
	}
}

func (self *Reader) Write(p []byte) (int, error) {
	self.buf = append(self.buf[:0], p...)
	return len(self.buf), nil
}

func (self *Reader) FixExtendedScanResponse() bool {
	fix := false
	for i := 4; i < len(self.buf)-1; i++ {
		if self.buf[i] == 0xdb && (self.buf[i+1] == 0xdc || self.buf[i+1] == 0xdd) {
			//log.Debugln(">>>", hex.EncodeToString(self.buf))
			self.buf = append(self.buf[:i], self.buf[i+1:]...)
			//log.Debugln("<<<", hex.EncodeToString(self.buf))
			fix = true
		}
	}
	return fix
}

type ScanResponse struct {
	RSSI       int8
	PacketType byte
	Addr       []byte
	AddrType   byte
	Bounding   byte
	DataLen    byte
	Data       []byte
}

func ParseScanResponse(data []byte) *ScanResponse {
	// check if evt_le_gap_scan_response
	if data[1]&0xA0 != 0xA0 || data[2] < 0x0b || data[3] != 3 || data[4] != 0 {
		return nil
	}
	response := &ScanResponse{
		RSSI:       int8(data[5]),
		PacketType: data[6],
		Addr:       data[7:13],
		AddrType:   data[13],
		Bounding:   data[14],
		DataLen:    data[15],
		Data:       data[16 : len(data)-1],
	}
	if (response.AddrType != 0 && response.AddrType != 1 && response.AddrType != 0xFF) ||
		int(response.DataLen) != len(response.Data) {
		return nil
	}
	return response
}

func ConvertExtendedToLegacy(extended []byte) int {
	// legacy payload is 7 bytes less
	legacy := make([]byte, 0, len(extended)-7)
	legacy = append(legacy, 0xC0, 0xA0, extended[2]-7, 3, 0)
	// rssi[18]
	legacy = append(legacy, extended[18])
	// packet_type[5], addr[6:12], addr_type[12], bonding[13]
	legacy = append(legacy, extended[5:14]...)
	// data_len[22], data
	legacy = append(legacy, extended[22:]...)
	return copy(extended, legacy)
}
