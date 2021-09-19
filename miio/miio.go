package miio

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"time"
)

const (
	BleQueryDev   = "_sync.ble_query_dev"
	GetProperties = "get_properties"
)

var requests = make(map[uint32]string)

func EncodeBind(addr uint8) []byte {
	return []byte(fmt.Sprintf(`{"method":"bind","address":%d}`, addr))
}

func EncodeBleQueryDev(mac string, pdid uint16) []byte {
	id := uint32(time.Now().Nanosecond()) & 0xFFFFFF
	requests[id] = BleQueryDev

	return []byte(fmt.Sprintf(
		`{"id":%d,"method":"%s","params":{"mac":"%s","pdid":%d}}`,
		id, BleQueryDev, mac, pdid,
	))
}

type Response struct {
	id uint32
}

func DecodeMethod(p []byte) string {
	payload := Response{}
	if err := json.Unmarshal(p, &payload); err != nil {
		log.Warn().Err(err).Send()
	} else {
		if method, ok := requests[payload.id]; ok {
			delete(requests, payload.id)
			return method
		}
	}
	return ""
}
