package bglib

import (
	"io"
)

type Reader struct {
	r io.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r}
}

func (self *Reader) Read(p []byte) (int, error) {
	buf := make([]byte, 260) // max payload size + 4
	n := 0
	chunk := make([]byte, 1)
	expectedLen := 0

	waitDB := true

	for {
		_, err := self.r.Read(chunk)
		if err != nil {
			return 0, err
		}

		// type byte = 0xA0 or 0x20
		if n == 0 && (chunk[0]&0x78 != 0x20) {
			return copy(p, chunk[:1]), nil
		}

		// new firmware has a bug with DB/DC or DB/DD bytes in payload in place of only one byte
		if waitDB {
			if n > 4 && buf[2] == 0x03 && buf[3] == 0x04 && chunk[0] == 0xDB {
				waitDB = false
			}
		} else {
			waitDB = true
			if chunk[0] == 0xDC || chunk[0] == 0xDD {
				continue
			}
		}

		buf[n] = chunk[0]
		n++

		switch n {
		case 2:
			// error
			if buf[1] == 0 {
				return copy(p, buf[:2]), nil
			}
			// cmdType + payloadLen + classID + commandID + payload
			expectedLen = 4 + int(buf[1])

		case expectedLen:
			return copy(p, buf[:expectedLen]), nil
		}
	}
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

func ConvertExtendedToLegacy(extended []byte) int {
	// legacy payload is 7 bytes less
	legacy := make([]byte, 0, len(extended)-7)
	legacy = append(legacy, 0xA0, extended[2]-7, 0x03, 0x00)
	// rssi[17]
	legacy = append(legacy, extended[17])
	// packet_type[4], addr[5:11], addr_type[11], bonding[12]
	legacy = append(legacy, extended[4:13]...)
	// data_len[21], data
	legacy = append(legacy, extended[21:]...)
	return copy(extended, legacy)
}
