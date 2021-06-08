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

func processExtResponse(data []byte) int {
	// save rssi before clear
	rssi := data[17]

	// clear rssi and channel before cache check
	data[17] = 0
	data[18] = 0

	// check if message in cache
	if cache.Test(string(data)) {
		return 0
	}

	// restore rssi
	data[17] = rssi

	n := bglib.ConvertExtendedToLegacy(data)
	msg := gap.ParseScanResponse(data[:n])

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
	case 0x004C: // iBeacon
		if msg.Raw[0xFF][2] == 2 && msg.Raw[0xFF][3] == 0x15 {
			msg.Data = gap.ParseIBeacon(msg.Raw[0xFF][2:])
			msg.Useful = 2
		}
	case 0x00D2: // Nut
		msg.Useful = 2
	case 0x0157: // MiBand or Amazfit Watch
		// don't know how to parse payload, but can be used as tracker
		msg.Useful = 2
	}

	if config.filter == 2 {
		// don't need raw payload in usual mode
		msg.Raw = nil
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

	return n
}

func uart2silabs(uart io.ReadWriteCloser, silabs io.ReadWriteCloser) {
	var skipBuf = make([]byte, 256)
	var skipN uint8

	var buf = make([]byte, 260) // max payload size + 4

	// bglib reader will return full command/event or return only 1 byte for wrong response bytes
	// fw v1.4.6_0012 returns 0x937162AD at start of each command/event
	// fw v1.5.0_0026 returns 0xC0 at start and at end of each command/event
	reader := bglib.NewReader(uart)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			log.Fatalln(err)
		}

		if n > 1 {
			// don't care if skip len lower than 6 bytes
			if skipN > 6 {
				log.Debugln("! Skip wrong bytes", hex.EncodeToString(skipBuf[:skipN]))
			}
			skipN = 0

			log.Traceln("=>", hex.EncodeToString(buf[:n]))
		} else {
			skipBuf[skipN] = buf[0]
			skipN++
		}

		// cmd_le_gap_set_discovery_extended_scan_response
		if n == 6 && buf[0] == 0x20 && buf[1] == 0x02 && buf[2] == 0x03 && buf[3] == 0x1C {
			log.Debugln("Get extended scan result")
			waitResponse <- true
			continue
		}

		// evt_le_gap_extended_scan_response
		if n >= 22 && buf[0] == 0xA0 && buf[1] >= 0x12 && buf[2] == 0x03 && buf[3] == 0x04 {
			// check adv len
			if buf[21] == byte(n)-22 {
				n = processExtResponse(buf[:n])
			} else {
				log.Debugln("! Wrong scan response len", hex.EncodeToString(buf[:n]))
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

		if n >= 3 && buf[2] == 0x03 && buf[3] == 0x16 {
			log.Infoln("Change interval and window to 10")
			binary.LittleEndian.PutUint16(buf[5:], 0x10)
			binary.LittleEndian.PutUint16(buf[7:], 0x10)
		}

		// before start cmd
		if n >= 3 && buf[2] == 0x03 && buf[3] == 0x18 {
			log.Infoln("Set extended scan")

			// send cmd_le_gap_set_discovery_extended_scan_response(enabled=1)
			buf2 := []byte{0x20, 0x01, 0x03, 0x1C, 0x01}
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
