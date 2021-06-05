package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	proto "github.com/huin/mqtt"
	log "github.com/sirupsen/logrus"
	"gw3/bglib"
	"gw3/gap"
	"io"
	"time"
)

var (
	cache        = NewCache()
	waitResponse = make(chan bool, 1)
)

type Cache struct {
	cache map[string]time.Time
	clear time.Time
}

func NewCache() *Cache {
	return &Cache{cache: make(map[string]time.Time), clear: time.Now()}
}

func (self *Cache) Test(key string) bool {
	now := time.Now()
	if self.clear.After(now) {
		for k, v := range self.cache {
			if now.After(v) {
				delete(self.cache, k)
			}
		}
		// clear cache once per minute
		self.clear = now.Add(time.Minute)
	}

	if ts, ok := self.cache[key]; ok && now.Before(ts) {
		return true
	}

	// put key in cache on 5 seconds
	self.cache[key] = now.Add(time.Second * 5)

	return false
}

// evt_le_gap_extended_scan_response
func processExtResponse(data []byte) int {
	// save rssi before clear
	rssi := data[18]

	// clear rssi and channel before cache check
	data[18] = 0
	data[19] = 0

	// check if message in cache
	if cache.Test(string(data)) {
		return 0
	}

	// restore rssi
	data[18] = rssi

	n := bglib.ConvertExtendedToLegacy(data)

	msg := gap.ParseScanResponse(data[1 : n-1])
	if msg != nil {
		switch msg.ServiceUUID {
		case 0x181A:
			msg.Data = gap.ParseATC1441(msg.Raw[0x16][2:])
			msg.Useful = 2

		case 0x181B:
			msg.Data = gap.ParseMiScalesV2(msg.Raw[0x16][2:])
			msg.Useful = 2

		case 0x181D:
			msg.Data = gap.ParseMiScalesV1(msg.Raw[0x16][2:])
			msg.Useful = 2

		case 0xFE95:
			mibeacon, useful := gap.ParseMiBeacon(msg.Raw[0x16][2:], config.Bindkeys())
			msg.Data = mibeacon
			// is encrypted
			if useful == 1 {
				miioClient.BleQueryDev(mibeacon.Mac, mibeacon.Pdid)
			} else {
				msg.Useful = useful
			}
		}

		switch msg.CompanyID {
		case 0x004C:
			if msg.Raw[0xFF][2] == 2 && msg.Raw[0xFF][3] == 0x15 {
				msg.Data = gap.ParseIBeacon(msg.Raw[0xFF][2:])
				msg.Useful = 2
			}
		}

		payload, err := json.Marshal(msg)
		if err == nil {
			log.Traceln("Publish:", string(payload))

			if mqttClient != nil && msg.Useful >= byte(config.filter) {
				mqttPayload.Payload = proto.BytesPayload(payload)
				mqttClient.Publish(mqttPayload)
			}
		} else {
			log.Warnln("Can't convert to JSON", hex.EncodeToString(data))
		}
	} else {
		log.Warnln("Can't unpack gap", hex.EncodeToString(data))
	}

	return n
}

func uart2silabs(uart io.ReadWriteCloser, silabs io.ReadWriteCloser) {
	var buf = make([]byte, 1024)

	reader := bglib.NewReader(uart)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			log.Fatalln(err)
		}

		log.Traceln("=>", hex.EncodeToString(buf[:n]))

		messageType := buf[1] & 0xF8

		if messageType == 0x20 && buf[3] == 3 && buf[4] == 0x1C {
			log.Debugln("Get extended scan result")
			waitResponse <- true
			continue
		}

		if messageType == 0xA0 && buf[3] == 3 && buf[4] == 4 {
			n = processExtResponse(buf[:n])
			if n == 0 {
				continue
			}
		}

		_, err = silabs.Write(buf[:n])
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func silabs2uart(silabs io.ReadWriteCloser, uart io.ReadWriteCloser) {
	var buf = make([]byte, 1024)
	for {
		n, err := silabs.Read(buf)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		if buf[2] == 3 && buf[3] == 0x16 {
			log.Infoln("Change interval and window to 10")
			binary.LittleEndian.PutUint16(buf[5:], 0x10)
			binary.LittleEndian.PutUint16(buf[7:], 0x10)
		}

		// before start cmd
		if buf[2] == 3 && buf[3] == 0x18 {
			log.Infoln("Set extended scan")

			// send cmd_le_gap_set_discovery_extended_scan_response(enabled=1)
			buf2 := []byte{0x20, 1, 3, 0x1C, 1}
			_, _ = uart.Write(buf2)

			log.Traceln("<=", hex.EncodeToString(buf2))

			<-waitResponse
		}

		log.Traceln("<=", hex.EncodeToString(buf[:n]))

		_, err = uart.Write(buf[:n])
		if err != nil {
			log.Fatalln(err)
		}
	}
}
