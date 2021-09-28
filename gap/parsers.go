package gap

import (
	"encoding/binary"
	"encoding/hex"
)

type Map map[string]interface{}

func (m Map) IsEvent() bool {
	_, ok := m["action"]
	return ok
}

func ParseATC1441(b []byte) Map {
	// without len, 0x16 and 0x181A
	switch len(b) {
	case 13: // atc1441
		return Map{
			"temperature": float32(int16(binary.BigEndian.Uint16(b[6:]))) / 10,
			"humidity":    float32(b[8]),
			"battery":     b[9],
			"voltage":     binary.BigEndian.Uint16(b[10:]),
			"seq":         b[12],
		}
	case 15: // pvvx
		return Map{
			"temperature": float32(int16(binary.LittleEndian.Uint16(b[6:]))) / 100,
			"humidity":    float32(binary.LittleEndian.Uint16(b[8:])) / 100,
			"voltage":     binary.LittleEndian.Uint16(b[10:]),
			"battery":     b[12],
			"seq":         b[13],
		}
	}
	return nil
}

// ParseMiScalesV1
// https://github.com/G1K/EspruinoHub/blob/3f3946206b81ea700493621f61c6d6e380b4ff0d/lib/attributes.js#L104
func ParseMiScalesV1(b []byte) Map {
	if len(b) < 3 {
		return nil
	}

	result := Map{
		"action":     "weight",
		"stabilized": b[0]&0b100000 > 0,
		"removed":    b[0]&0b10000000 > 0,
	}

	weight := float32(binary.LittleEndian.Uint16(b[1:]))

	switch {
	case b[0]&0b10000 > 0:
		result["weight"] = weight / 100
	case b[0]&0b1 > 0:
		result["weight_lb"] = weight / 100
	default:
		result["weight_kg"] = weight / 200
	}

	return result
}

// ParseMiScalesV2
// https://github.com/G1K/EspruinoHub/blob/3f3946206b81ea700493621f61c6d6e380b4ff0d/lib/attributes.js#L78
func ParseMiScalesV2(b []byte) Map {
	if len(b) < 12 {
		return nil
	}

	result := Map{
		"action":     "weight",
		"stabilized": b[1]&0b100000 > 0,
		"removed":    b[1]&0b10000000 > 0,
	}

	if b[1]&0b10 > 0 {
		result["impedance"] = binary.LittleEndian.Uint16(b[9:])
	}

	weight := float32(binary.LittleEndian.Uint16(b[11:]))

	switch b[0] {
	case 0b10000:
		result["weight"] = weight / 100
	case 3:
		result["weight_lb"] = weight / 100
	case 2:
		result["weight_kg"] = weight / 200
	}

	return result
}

func ParseIBeacon(b []byte) Map {
	// https://support.kontakt.io/hc/en-gb/articles/201492492-iBeacon-advertising-packet-structure
	// 0      0x02
	// 1      0x15
	// 2..17  UUID (16 bytes)
	// 18 19  Major
	// 20 21  Minor
	// 22     Power
	if len(b) != 23 || b[0] != 0x02 || b[1] != 0x15 {
		return nil
	}

	return Map{
		"uuid":  hex.EncodeToString(b[2:18]),
		"major": binary.BigEndian.Uint16(b[18:]),
		"minor": binary.BigEndian.Uint16(b[20:]),
		"tx":    int8(b[22]),
	}
}
