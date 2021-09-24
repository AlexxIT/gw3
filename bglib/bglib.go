package bglib

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
)

var WrongRead = errors.New("")

const Cmd_system_reset = 0x2000_0101
const Cmd_system_get_bt_address = 0x2000_0103
const Cmd_le_gap_set_discovery_timing = 0x2000_0316
const Cmd_le_gap_start_discovery = 0x2000_0318
const Cmd_le_gap_set_discovery_extended_scan_response = 0x2000_031C
const Cmd_mesh_node_set_ivrecovery_mode = 0x2000_1406

const Evt_system_boot = 0xA000_0100
const Evt_le_gap_adv_timeout = 0xA000_0301
const Evt_le_gap_extended_scan_response = 0xA000_0304

type ChipReader struct {
	r io.Reader
}

func NewReader(r io.Reader) *ChipReader {
	return &ChipReader{r}
}

func (r *ChipReader) Read(p []byte) (int, error) {
	buf := make([]byte, 260) // max payload size + 4
	n := 0
	chunk := make([]byte, 1)
	expectedLen := 0

	isGap := false

	for {
		if _, err := r.r.Read(chunk); err != nil {
			return 0, err
		}

		// type byte = 0xA0 or 0x20
		if n == 0 && (chunk[0]&0x7F != 0x20) {
			return copy(p, chunk[:1]), WrongRead
		}

		// new firmware has a bug with DB/DC or DB/DD bytes in payload in place of only one byte
		// we should jump over this byte
		// a026030400______50ec5000ff0100ff7fdbdc26000014020106101695fe9055eb0601______50ec500e00
		//                                   ^^^^
		// a028030403______50ec5000ff0100ff7fb327000016152a6935020c0fefdbdd214e1e7768c729201c86fd36c7
		//                                                             ^^^^
		// a028030403______50ec5000ff0100ff7fbb27000016152a69760eef45d6d8e271866b11dbddcf03c8ee530d01
		//                                                                         ^^^^
		// a025030400______38c1a400ff0100ff7fd12700001312161a18______38c1a440086714060b45dbdc0f c0c0
		//                                                                               ^^^^ last byte
		if isGap && buf[n-1] == 0xDB && (chunk[0] == 0xDC || chunk[0] == 0xDD) {
			continue
		}

		buf[n] = chunk[0]
		n++

		switch n {
		case 2:
			// length should be greater than 0
			if buf[1] == 0 {
				return copy(p, buf[:2]), WrongRead
			}
			// cmdType + payloadLen + classID + commandID + payload
			expectedLen = 4 + int(buf[1])

		case expectedLen:
			return copy(p, buf[:expectedLen]), nil

		case 5:
			// default: will check byte 5 and byte 6
			isGap = buf[0] == 0xA0 && buf[2] == 0x03 && buf[3] == 0x04

		case 16:
			// sometimes data has a bug for evt_le_gap_extended_scan_response
			// buf[15] = 0xC0 (new message) but always should be 0xFF
			if buf[0] == 0xA0 && buf[2] == 0x03 && buf[3] == 0x04 && buf[15] != 0xFF {
				return copy(p, buf[:16]), WrongRead
			}

		case 260:
			// this shouldn't happen
			log.Panic().Hex("data", buf[:260]).Int("len", expectedLen).Send()
		}
	}
}

func IsResetCmd(p []byte) bool {
	return bytes.Compare(p, []byte{0x20, 1, 1, 1, 0}) == 0
}

type Map map[string]interface{}

func DecodeResponse(p []byte, n int) (uint32, Map) {
	header := uint32(p[0])<<24 | uint32(p[2])<<8 | uint32(p[3])
	switch header {
	case Cmd_system_get_bt_address:
		if n == 10 && p[1] == 0x06 {
			mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", p[9], p[8], p[7], p[6], p[5], p[4])
			return header, Map{"mac": mac}
		}
	case Cmd_le_gap_set_discovery_extended_scan_response:
		if n == 6 && p[1] == 0x02 {
			return header, nil
		}
	case Cmd_le_gap_start_discovery:
		if n == 6 && p[1] == 0x02 {
			return header, nil
		}
	case Evt_system_boot:
		if n == 22 && p[1] == 0x12 {
			return header, nil
		}
	case Evt_le_gap_extended_scan_response:
		if n >= 22 && p[1] >= 0x12 && p[21] == byte(n)-22 {
			return header, nil
		} else {
			log.Debug().Hex("data", p[:n]).Msg("! Wrong scan response len")
		}
	}
	return 0, nil
}

func PatchGapDiscoveryTiming(p []byte, scanInterval uint16, scanWindow uint16) {
	// cmd_le_gap_set_discovery_timing
	binary.LittleEndian.PutUint16(p[5:], scanInterval)
	binary.LittleEndian.PutUint16(p[7:], scanWindow)
}

func EncodeGapExtendedScan(enabled uint8) []byte {
	// cmd_le_gap_set_discovery_extended_scan_response
	return []byte{0x20, 0x01, 0x03, 0x1C, enabled}
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

func ParseVersion(b []byte) string {
	maj := binary.LittleEndian.Uint16(b[4:])
	min := binary.LittleEndian.Uint16(b[6:])
	pat := binary.LittleEndian.Uint16(b[8:])
	return fmt.Sprintf("%d.%d.%d", maj, min, pat)
}

func ParseProvInit(b []byte) (addr uint16, ivi uint32) {
	return binary.LittleEndian.Uint16(b[5:]), binary.LittleEndian.Uint32(b[7:])
}

func ParseDeviceInfo(b []byte) (uuid []byte, address uint16, elements uint8) {
	return b[4:20], binary.LittleEndian.Uint16(b[20:]), b[22]
}

//func ParseMeshReceive(b []byte) (address uint16, payload []byte) {
//	l := 20
//	return binary.LittleEndian.Uint16(b[10:]), b[l:]
//}

type MeshVendorModel struct {
	VendorID   uint16
	ModelID    uint16
	SourceAddr uint16
	Opcode     uint8
	Payload    []byte
}

func DecodeMeshVendorModel(b []byte) *MeshVendorModel {
	return &MeshVendorModel{
		VendorID:   binary.LittleEndian.Uint16(b[6:]),
		ModelID:    binary.LittleEndian.Uint16(b[8:]),
		SourceAddr: binary.LittleEndian.Uint16(b[10:]),
		Opcode:     b[18],
		Payload:    b[21:],
	}
}

// MeshStatus evt_mesh_generic_client_server_status
type MeshStatus struct {
	ModelID    uint16 `json:"mid"`
	ElemIndex  uint16 `json:"eid"`
	ClientAddr uint16 `json:"caddr"`
	ServerAddr uint16 `json:"saddr"`
	Remain     uint32 `json:"remain"`
	Flags      uint16 `json:"flags"`
	Type       uint8  `json:"type"`
	Params     []byte `json:"params"`
}

func ParseMeshStatus(b []byte) *MeshStatus {
	return &MeshStatus{
		ModelID:    binary.LittleEndian.Uint16(b[4:]),
		ElemIndex:  binary.LittleEndian.Uint16(b[6:]),
		ClientAddr: binary.LittleEndian.Uint16(b[8:]),
		ServerAddr: binary.LittleEndian.Uint16(b[10:]),
		Remain:     binary.LittleEndian.Uint32(b[12:]),
		Flags:      binary.LittleEndian.Uint16(b[16:]),
		Type:       b[18],
		Params:     b[20:],
	}
}

func ParseIVRecovery(b []byte) (nodeIndex uint32, networkIndex uint32) {
	return binary.LittleEndian.Uint32(b[4:]), binary.LittleEndian.Uint32(b[8:])
}

func ParseIVUpdateState(b []byte) (result uint16, ivi uint32, state uint8) {
	return binary.LittleEndian.Uint16(b[4:]), binary.LittleEndian.Uint32(b[6:]), b[10]
}

func EncodeGenericClientSet(modelID uint16, serverAddr uint16, type_ uint8, payload interface{}, transition uint32) []byte {
	b := []byte{
		0x20, 0, 0x1E, 1, // cmd_mesh_generic_client_set
		0, 0, // model_id
		0, 0, // elem_index
		0, 0, // server_address
		0, 0, // appkey_index
		0,          // tid
		0, 0, 0, 0, // transition
		0, 0, // delay
		1, 0, // flags (response required)
		type_, // type
	}
	binary.LittleEndian.PutUint16(b[4:], modelID)
	binary.LittleEndian.PutUint16(b[8:], serverAddr)
	binary.LittleEndian.PutUint32(b[13:], transition)
	switch payload.(type) {
	case uint8:
		b = append(b, 1, payload.(uint8))
	case uint16:
		b = append(b, 2, 0, 0)
		binary.LittleEndian.PutUint16(b[23:], payload.(uint16))
	case uint32:
		b = append(b, 4, 0, 0, 0, 0)
		binary.LittleEndian.PutUint32(b[23:], payload.(uint32))
	case []byte:
		l := len(payload.([]byte))
		b = append(b, uint8(l))
		b = append(b, payload.([]byte)...)
	}
	b[1] = uint8(len(b) - 4)
	return b
}

func EncodeGenericClientGet(modelID uint16, serverAddr uint16, type_ uint8) []byte {
	b := make([]byte, 13)
	b[0] = 0x20 // command
	b[1] = 9    // main payload len
	b[2] = 0x1E // cmd_mesh_generic_client_get
	binary.LittleEndian.PutUint16(b[4:], modelID)
	binary.LittleEndian.PutUint16(b[8:], serverAddr)
	b[12] = type_
	return b
}

func EncodeMeshVendorModelSend(addr uint16, payload []byte) []byte {
	l := uint8(19 - 4 + len(payload))
	b := []byte{
		0x20, l, 0x19, 0, // cmd_mesh_vendor_model_send
		0, 0, // elem_index
		0x8F, 3, // vendor_id
		1, 0, // model_id
		0, 0, // destination_address
		0,    // va_index
		0, 0, // appkey_index
		0, // nonrelayed
		3, // opcode
		1, // final
		uint8(len(payload)),
	}
	binary.LittleEndian.PutUint16(b[10:], addr)
	b = append(b, payload...)
	return b
}

func EncodeMeshConfigClientListSubs(addr uint16) []byte {
	b := []byte{
		0x20, 0x09, 0x27, 0x15, // cmd_mesh_config_client_list_subs
		0, 0, // enc_netkey_index
		0, 0, // server_address
		0,          // element_index
		0xFF, 0xFF, // vendor_id (use 0xFFFF for Bluetooth SIG models)
		0, 0x10, // model_id
	}
	binary.LittleEndian.PutUint16(b[6:], addr)
	return b
}

var ParseError = errors.New("parse error")

// cmd_system_get_bt_address
func DecodeSystemGetBTAddress(b []byte) ([]byte, error) {
	if len(b) != 10 || b[1] != 0x06 {
		return nil, ParseError
	}
	return b[4:], nil
}

// cmd_le_gap_set_discovery_extended_scan_response
func DecodeLeGapSetDiscoveryExtendedScanResponse(b []byte) error {
	if len(b) != 6 || b[1] != 0x02 {
		return ParseError
	}
	return nil
}

// cmd_mesh_node_get_ivupdate_state
func DecodeMeshNodeGetIVUpdateState(b []byte) (result uint16, ivi uint32, state uint8, err error) {
	if len(b) != 11 || b[1] != 0x07 {
		return 0, 0, 0, ParseError
	}
	return binary.LittleEndian.Uint16(b[4:]), binary.LittleEndian.Uint32(b[6:]), b[10], nil
}

// cmd_mesh_config_client_list_subs
func DecodeMeshConfigClientListSubs(b []byte) (uint32, error) {
	if len(b) != 10 || b[4] != 0 || b[5] != 0 {
		return 0, ParseError
	}
	return binary.LittleEndian.Uint32(b[6:]), nil
}

// evt_mesh_config_client_subs_list
func DecodeMeshConfigClientSubsList(b []byte) (uint32, []uint16, error) {
	n := uint8(len(b))
	if n < 9 || b[1] < 5 || b[8] != n-9 {
		return 0, nil, ParseError
	}
	n = b[8] / 2
	groups := make([]uint16, n)
	for i := uint8(0); i < n; i++ {
		groups[i] = binary.LittleEndian.Uint16(b[9+i*2:])
	}
	handle := binary.LittleEndian.Uint32(b[4:])
	return handle, groups, nil
}
