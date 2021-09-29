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
	Id uint32 `json:"id"`
}

func DecodeMethod(p []byte) string {
	payload := Response{}
	if err := json.Unmarshal(p, &payload); err != nil {
		log.Warn().Err(err).Send()
	} else {
		if method, ok := requests[payload.Id]; ok {
			delete(requests, payload.Id)
			return method
		}
	}
	return ""
}
