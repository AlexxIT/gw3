package gap

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type Message struct {
	PacketType byte   `json:"type"`
	MAC        string `json:"mac"`
	Rand       byte   `json:"rand"`
	RSSI       int8   `json:"rssi"`

	Brand   string `json:"brand,omitempty"`
	Name    string `json:"name,omitempty"`
	Comment string `json:"comment,omitempty"`
	Useful  byte   `json:"useful"`

	// https://btprodspecificationrefs.blob.core.windows.net/assigned-values/16-bit%20UUID%20Numbers%20Document.pdf
	ServiceUUID uint16 `json:"uuid,omitempty"`
	// https://www.bluetooth.com/specifications/assigned-numbers/company-identifiers/
	CompanyID uint16            `json:"cid,omitempty"`
	Raw       map[byte]hexbytes `json:"raw,omitempty"`

	Data Map `json:"data,omitempty"`
}

type hexbytes []byte

func (h hexbytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(h))
}

type Service struct {
	UUID string `json:"uuid"`
}

// Brands
// https://www.bluetooth.com/specifications/assigned-numbers/company-identifiers/
var Brands = map[uint16]string{
	0x0006: "Microsoft",
	0x004C: "Apple",
	0x0075: "Samsung",
	0x00E0: "Google",
	0x0157: "Huami",
	0x05A7: "Sonos",
}

func SprintMAC(b []byte) string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[5], b[4], b[3], b[2], b[1], b[0])
}

func ParseScanResponse(data []byte) *Message {
	msg := &Message{
		PacketType: data[5],
		MAC:        SprintMAC(data[6:12]),
		Rand:       data[12],
		RSSI:       int8(data[4]),
		Raw:        make(map[byte]hexbytes),
	}

	data = data[15:]

	var i int
	for i < len(data) {
		// 1 byte | len
		// 1 byte | advType
		// 2 byte | serviceID (advType=0x16) or company ID (advType=0xFF)
		// X byte | other data (len-3 bytes)
		l := int(data[i])
		if l < 2 || i+l >= len(data) {
			msg.Comment = "wrong len"
			msg.Useful = 0
			return msg
		}

		advType := data[i+1]
		if (0x2D < advType && advType < 0xFF) || advType == 0 {
			msg.Comment = "wrong adv type"
			msg.Useful = 0
			return msg
		}
		msg.Raw[advType] = data[i+2 : i+l+1]

		switch advType {
		case 0x08, 0x09:
			msg.Name = string(data[i+2 : i+l+1])
		case 0x16:
			msg.ServiceUUID = binary.LittleEndian.Uint16(data[i+2:])
			switch msg.ServiceUUID {
			case 0xFE95:
				msg.Brand = "Xiaomi"
				msg.Useful = 1
			case 0xFE9F:
				msg.Brand = "Google"
				msg.Useful = 0
			default:
				msg.Useful = 1
			}
		case 0x2A:
			msg.Comment = "Mesh Message"
			msg.Useful = 0
		case 0x2B:
			msg.Comment = "Mesh Beacon"
			msg.Useful = 0
		case 0xFF:
			msg.CompanyID = binary.LittleEndian.Uint16(data[i+2:])
			if val, ok := Brands[msg.CompanyID]; ok {
				msg.Brand = val
			}
			msg.Useful = 1
		}

		i += 1 + l
	}
	if i != len(data) {
		msg.Comment = "wrong len"
	}
	return msg
}
