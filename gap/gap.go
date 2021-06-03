package gap

import (
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gw3/crypt"
)

type MiBeacon struct {
	Mac     string `json:"mac"`
	Pdid    uint16 `json:"pdid"`
	Eid     uint16 `json:"eid,omitempty"`
	Edata   string `json:"edata,omitempty"`
	Seq     byte   `json:"seq"`
	Comment string `json:"comment,omitempty"`
}

type GAPMessage struct {
	PacketType byte   `json:"type"`
	Address    string `json:"addr"`
	AddrRand   byte   `json:"rand"`
	RSSI       int8   `json:"rssi"`

	Brand   string `json:"brand,omitempty"`
	Comment string `json:"comment,omitempty"`
	Useful  byte   `json:"useful"`

	ServiceUUID uint16 `json:"suuid,omitempty"`
	Raw         RawGAP `json:"raw"`

	Data interface{} `json:"data,omitempty"`
}

type Bindkeys map[string]string
type RawGAP map[byte][]byte

func (self RawGAP) MarshalJSON() ([]byte, error) {
	var i int
	var res string
	for k, v := range self {
		res += fmt.Sprintf("%02X:%s", k, hex.EncodeToString(v))
		if i++; i < len(self) {
			res += " "
		}
	}
	return json.Marshal(res)
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

func ParseScanResponse(data []byte) *GAPMessage {
	msg := &GAPMessage{
		PacketType: data[5],
		Address:    SprintMAC(data[6:12]),
		AddrRand:   data[12],
		RSSI:       int8(data[4]),
		Raw:        make(RawGAP),
	}

	data = data[15:]

	var i int
	for i < len(data) {
		l := int(data[i])
		if i+l >= len(data) {
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
			manufID := binary.LittleEndian.Uint16(data[i+2:])
			if val, ok := Brands[manufID]; ok {
				msg.Brand = val
			} else {
				msg.Brand = fmt.Sprintf("0x%04X", manufID)
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

func ParseMiBeacon(data []byte, keys Bindkeys) (mibeacon *MiBeacon, useful byte) {
	// https://iot.mi.com/new/doc/embedded-development/ble/ble-mibeacon
	mibeacon = &MiBeacon{
		Pdid: binary.LittleEndian.Uint16(data[2:]),
		Seq:  data[4],
	}

	frame := binary.LittleEndian.Uint16(data)
	// check mac
	if frame&0x10 == 0 {
		mibeacon.Comment = "no mac"
		return mibeacon, 0
	}

	mibeacon.Mac = SprintMAC(data[5:])

	// check payload
	if frame&0x40 == 0 {
		mibeacon.Comment = "no payload"
		return mibeacon, 0
	}

	version := (frame >> 12) & 0b1111

	i := 5 + 6

	// check capability
	if frame&0x20 > 0 {
		capab := data[i]
		i++
		if (capab >> 3) == 0b11 {
			i += 2
		}
		if version == 5 && capab&0x20 > 0 {
			i += 2
		}
	}

	var payload []byte

	// check encryption
	if frame&0x08 > 0 {
		// keys can be nil, no problem
		if key, ok := keys[mibeacon.Mac]; ok {
			switch version {
			case 2, 3:
				payload = MiBeaconDecode1(data, i, key)
			case 4, 5:
				payload = MiBeaconDecode4(data, i, key)
			}
			if payload == nil {
				mibeacon.Comment = "wrong enc key"
				return mibeacon, 1
			}
		} else {
			mibeacon.Comment = "encrypted"
			return mibeacon, 1
		}
	} else if version == 5 && frame&0x80 > 0 {
		payload = data[i : len(data)-2]
	} else {
		payload = data[i:]
	}

	if len(payload) < 4 {
		mibeacon.Edata = hex.EncodeToString(payload)
		mibeacon.Comment = "small payload"
		return mibeacon, 0
	}

	// skip payload len check because ATC_MiThermometer has wrong payload
	//if payload[2] != byte(len(payload))-3 {
	//	mibeacon.Edata = hex.EncodeToString(payload])
	//	mibeacon.Comment = "wrong len payload"
	//	return mibeacon, 1
	//}

	mibeacon.Eid = binary.LittleEndian.Uint16(payload)
	mibeacon.Edata = hex.EncodeToString(payload[3:])

	return mibeacon, 2
}

func MiBeaconDecode1(mibeacon []byte, pos int, key string) []byte {
	key2, _ := hex.DecodeString(key)
	key3 := make([]byte, 0, 16)
	key3 = append(key3, key2[:6]...)
	key3 = append(key3, 0x8d, 0x3d, 0x3c, 0x97)
	key3 = append(key3, key2[6:12]...)
	c, err := aes.NewCipher(key3)
	if err != nil {
		return nil
	}

	nonce := make([]byte, 0, 13)
	// frame + pdid + cnt
	nonce = append(nonce, mibeacon[0:5]...)
	// counter
	nonce = append(nonce, mibeacon[len(mibeacon)-4:len(mibeacon)-1]...)
	// mac5
	nonce = append(nonce, mibeacon[5:10]...)

	// witout tag validating, because tag only 1 byte len
	ccm, err := crypt.NewCCMWithNonceAndTagSizes(c, len(nonce), 0)
	if err != nil {
		return nil
	}

	ciphertext := mibeacon[pos : len(mibeacon)-4]

	plain, err := ccm.Open(nil, nonce, ciphertext, []byte{0x11})
	if err != nil {
		return nil
	}

	return plain
}

func MiBeaconDecode4(mibeacon []byte, pos int, key string) []byte {
	key2, _ := hex.DecodeString(key)
	c, err := aes.NewCipher(key2)
	if err != nil {
		return nil
	}

	nonce := make([]byte, 0, 12)
	// mac
	nonce = append(nonce, mibeacon[5:11]...)
	// pdid + seq
	nonce = append(nonce, mibeacon[2:5]...)
	// counter
	nonce = append(nonce, mibeacon[len(mibeacon)-7:len(mibeacon)-4]...)

	ccm, err := crypt.NewCCMWithNonceAndTagSizes(c, len(nonce), 4)
	if err != nil {
		return nil
	}

	ciphertext := mibeacon[pos : len(mibeacon)-7]
	// cipertext should contain token/tag at the end (4 bytes)
	ciphertext = append(ciphertext, mibeacon[len(mibeacon)-4:]...)

	plain, err := ccm.Open(nil, nonce, ciphertext, []byte{0x11})
	if err != nil {
		return nil
	}

	return plain
}
