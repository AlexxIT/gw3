package gap

import (
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/AlexxIT/gw3/crypt"
)

type MiBeacon struct {
	Mac     string   `json:"mac"`
	Pdid    uint16   `json:"pdid"`
	Eid     uint16   `json:"eid,omitempty"`
	Edata   hexbytes `json:"edata,omitempty"`
	Seq     byte     `json:"seq"`
	Comment string   `json:"comment,omitempty"`
}

func (b *MiBeacon) Decode() Map {
	switch b.Eid {
	case 0:
		return Map{}
	case 0x1001: // 4097
		if len(b.Edata) == 3 {
			// TODO: remotes...
			value := fmt.Sprintf("%d", b.Edata[0])
			return Map{"action": value}
		}
	case 0x1002: // 4098
		// No sleep (0x00), falling asleep (0x01)
		return Map{"sleep": b.Edata[0]}
	case 0x1003: // 4099
		return Map{"rssi": int8(b.Edata[0])}
	case 0x1004: // 4100
		if len(b.Edata) == 2 {
			value := float32(int16(binary.LittleEndian.Uint16(b.Edata))) / 10
			return Map{"temperature": value}
		}
	case 0x1005: // 4101
		if len(b.Edata) == 2 {
			// Kettle, thanks https://github.com/custom-components/ble_monitor/
			return Map{"power": b.Edata[0], "temperature": float32(b.Edata[1])}
		}
	case 0x1006: // 4102
		if len(b.Edata) == 2 {
			value := float32(binary.LittleEndian.Uint16(b.Edata)) / 10
			if b.Pdid == 903 || b.Pdid == 1371 {
				// two models has bug, they increase humidity on each data by 0.1
				value = float32(int32(value))
			}
			return Map{"humidity": value}
		}
	case 0x1007: // 4103
		if len(b.Edata) == 3 {
			value := uint32(b.Edata[0]) | uint32(b.Edata[1])<<8 | uint32(b.Edata[2])<<16
			if b.Pdid == 2038 {
				// Night Light 2: 1 - no light, 100 - light
				if value >= 100 {
					return Map{"light": 1}
				} else {
					return Map{"light": 0}
				}
			}
			// Range: 0-120000, lux
			return Map{"illuminance": value}
		}
	case 0x1008: // 4104
		// Humidity percentage, range: 0-100
		return Map{"moisture": b.Edata[0]}
	case 0x1009: // 4105
		if len(b.Edata) == 2 {
			// Soil EC value, Unit us/cm, range: 0-5000
			value := binary.LittleEndian.Uint16(b.Edata)
			return Map{"conductivity": value}
		}
	case 0x100A: // 4106
		return Map{"battery": b.Edata[0]}
	case 0x100D: // 4109
		if len(b.Edata) == 4 {
			value1 := float32(int16(binary.LittleEndian.Uint16(b.Edata))) / 10
			value2 := float32(binary.LittleEndian.Uint16(b.Edata[2:])) / 10
			return Map{"temperature": value1, "humidity": value2}
		}
	case 0x100E: // 4110
		// 1 => true => on => unlocked
		if b.Edata[0] == 0 {
			return Map{"lock": 1}
		} else {
			return Map{"lock": 0}
		}
	case 0x100F: // 4111
		// 1 => true => on => dooor opening
		if b.Edata[0] == 0 {
			return Map{"opening": 1}
		} else {
			return Map{"opening": 0}
		}
	case 0x1010: // 4112
		if len(b.Edata) == 2 {
			value := float32(int16(binary.LittleEndian.Uint16(b.Edata))) / 100
			return Map{"formaldehyde": value}
		}
	case 0x1012: // 4114
		// 1 => true => open
		return Map{"opening": b.Edata[0]}
	case 0x1013: // 4115
		// Remaining percentage, range 0~100
		return Map{"supply": b.Edata[0]}
	case 0x1014: // 4116
		// 1 => on => wet
		return Map{"water_leak": b.Edata[0]}
	case 0x1015: // 4117
		// 1 => on => alarm
		return Map{"smoke": b.Edata[0]}
	case 0x1016: // 4118
		// 1 => on => alarm
		return Map{"gas": b.Edata[0]}
	case 0x1017: // 4119
		if len(b.Edata) == 4 {
			// The duration of the unmanned state, in seconds
			value := binary.LittleEndian.Uint32(b.Edata)
			return Map{"idle_time": value}
		}
	case 0x1018: // 4120
		// Door Sensor 2: 0 - dark, 1 - light
		return Map{"light": b.Edata[0]}
	case 0x1019: // 4121
		// 0x00: open the door, 0x01: close the door,
		// 0x02: not closed after timeout, 0x03: device reset
		// 1 => true => open
		switch b.Edata[0] {
		case 0:
			return Map{"contact": 1}
		case 1:
			return Map{"contact": 0}
		}
	case 0x06:
		if len(b.Edata) == 5 {
			actionID := b.Edata[4]
			keyID := binary.LittleEndian.Uint32(b.Edata)
			return Map{
				"action":    "fingerprint",
				"action_id": actionID,
				"key_id":    fmt.Sprintf("%04x", keyID),
				"message":   mibeaconFingerprintAction(actionID),
			}
		}
	case 0x07:
		actionID := b.Edata[0]
		return Map{
			"action":    "door",
			"action_id": actionID,
			"message":   mibeaconDoorAction(actionID),
		}
	case 0x0008:
		if b.Edata[0] > 0 {
			return Map{"action": "armed", "state": true}
		} else {
			return Map{"action": "armed", "state": false}
		}
	case 0x0B: // 11
		var keyID string
		actionID := b.Edata[0] & 0xF
		methodID := b.Edata[0] >> 4
		key := binary.LittleEndian.Uint32(b.Edata[1:])
		err := mibeaconLockError(key)
		if err == "" && methodID > 0 {
			keyID = fmt.Sprintf("%d", key&0xFFFF)
		} else {
			keyID = fmt.Sprintf("%04x", key)
		}
		timestamp := binary.LittleEndian.Uint32(b.Edata[5:])
		return Map{
			"action":    "lock",
			"action_id": actionID,
			"method_id": methodID,
			"message":   mibeaconLockAction(actionID),
			"method":    mibeaconLockMethod(methodID),
			"key_id":    keyID,
			"error":     err,
			"timestamp": timestamp,
		}
	case 0x0F: // 15
		if len(b.Edata) == 3 {
			// Night Light 2: 1 - moving no light, 100 - moving with light
			// Motion Sensor 2: 0 - moving no light, 256 - moving with light
			// Qingping Motion Sensor - moving with illuminance data
			value := uint32(b.Edata[0]) | uint32(b.Edata[1])<<8 | uint32(b.Edata[2])<<16
			if b.Pdid == 2691 {
				return Map{"action": "motion", "motion": 1, "illuminance": value}
			} else if value >= 100 {
				return Map{"action": "motion", "motion": 1, "light": 1}
			} else {
				return Map{"action": "motion", "motion": 1, "light": 0}
			}
		}
	case 0x10:
		if len(b.Edata) == 2 {
			// Toothbrush Т500
			if b.Edata[0] == 0 {
				return Map{"action": "start", "counter": b.Edata[1]}
			} else {
				return Map{"action": "finish", "score": b.Edata[1]}
			}
		}
	}
	return Map{"raw": b}
}

func ParseMiBeacon(data []byte, getBindkey func(string) string) (mibeacon *MiBeacon, useful byte) {
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
		key := getBindkey(mibeacon.Mac)
		if key != "" {
			switch version {
			case 2, 3:
				payload = mibeaconDecode1(data, i, key)
			case 4, 5:
				payload = mibeaconDecode4(data, i, key)
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
		mibeacon.Edata = payload
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
	mibeacon.Edata = payload[3:]

	return mibeacon, 2
}

func mibeaconDecode1(mibeacon []byte, pos int, key string) []byte {
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

func mibeaconDecode4(mibeacon []byte, pos int, key string) []byte {
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

func mibeaconFingerprintAction(actionID uint8) string {
	switch actionID {
	case 0:
		return "Match successful"
	case 1:
		return "Match failed"
	case 2:
		return "Timeout"
	case 3:
		return "Low quality"
	case 4:
		return "Insufficient area"
	case 5:
		return "Skin is too dry"
	case 6:
		return "Skin is too wet"
	}
	return ""
}

func mibeaconDoorAction(actionID uint8) string {
	switch actionID {
	case 0:
		return "Door is open"
	case 1:
		return "Door is closed"
	case 2:
		return "Timeout is not closed"
	case 3:
		return "Knock on the door"
	case 4:
		return "Breaking the door"
	case 5:
		return "Door is stuck"
	}
	return ""
}

func mibeaconLockAction(actionID uint8) string {
	switch actionID {
	case 0b0000:
		return "Unlock outside the door"
	case 0b0001:
		return "Lock"
	case 0b0010:
		return "Turn on anti-lock"
	case 0b0011:
		return "Turn off anti-lock"
	case 0b0100:
		return "Unlock inside the door"
	case 0b0101:
		return "Lock inside the door"
	case 0b0110:
		return "Turn on child lock"
	case 0b0111:
		return "Turn off child lock"
	case 0b1111:
		return "-"
	}
	return ""
}

func mibeaconLockMethod(methodID uint8) string {
	switch methodID {
	case 0b0000:
		return "bluetooth"
	case 0b0001:
		return "password"
	case 0b0010:
		return "biological"
	case 0b0011:
		return "key"
	case 0b0100:
		return "turntable"
	case 0b0101:
		return "nfc"
	case 0b0110:
		return "one-time password"
	case 0b0111:
		return "two-step verification"
	case 0b1000:
		return "coercion"
	case 0b1010:
		return "manual"
	case 0b1011:
		return "automatic"
	case 0b1111:
		return "-"
	}
	return ""
}

func mibeaconLockError(keyID uint32) string {
	switch keyID {
	case 0xC0DE0000:
		return "Frequent unlocking with incorrect password"
	case 0xC0DE0001:
		return "Frequent unlocking with wrong fingerprints"
	case 0xC0DE0002:
		return "Operation timeout (password input timeout)"
	case 0xC0DE0003:
		return "Lock picking"
	case 0xC0DE0004:
		return "Reset button is pressed"
	case 0xC0DE0005:
		return "The wrong key is frequently unlocked"
	case 0xC0DE0006:
		return "Foreign body in the keyhole"
	case 0xC0DE0007:
		return "The key has not been taken out"
	case 0xC0DE0008:
		return "Error NFC frequently unlocks"
	case 0xC0DE0009:
		return "Timeout is not locked as required"
	case 0xC0DE000A:
		return "Failure to unlock frequently in multiple ways"
	case 0xC0DE000B:
		return "Unlocking the face frequently fails"
	case 0xC0DE000C:
		return "Failure to unlock the vein frequently"
	case 0xC0DE000D:
		return "Hijacking alarm"
	case 0xC0DE000E:
		return "Unlock inside the door after arming"
	case 0xC0DE000F:
		return "Palmprints frequently fail to unlock"
	case 0xC0DE0010:
		return "The safe was moved"
	case 0xC0DE1000:
		return "The battery level is less than 10%"
	case 0xC0DE1001:
		return "The battery is less than 5%"
	case 0xC0DE1002:
		return "The fingerprint sensor is abnormal"
	case 0xC0DE1003:
		return "The accessory battery is low"
	case 0xC0DE1004:
		return "Mechanical failure"
	}
	return ""
}
