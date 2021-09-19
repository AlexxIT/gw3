package main

import (
	"fmt"
	"github.com/AlexxIT/gw3/bglib"
	"github.com/AlexxIT/gw3/gap"
	"github.com/AlexxIT/gw3/serial"
	"github.com/rs/zerolog/log"
	"io"
	"time"
)

var btchip io.ReadWriteCloser

// btchipInit open serial connection to /dev/ttyS1
func btchipInit() {
	var err error
	btchip, err = serial.Open(serial.OpenOptions{
		PortName:        "/dev/ttyS1",
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
		//RTSCTSFlowControl: true,
	})
	if err != nil {
		log.Fatal().Err(err).Send()
	}
}

// btchipReader loops reading data from BT chip
func btchipReader() {
	var p = make([]byte, 260) // max payload size + 4

	var skipBuf = make([]byte, 256)
	var skipN uint8

	// bglib reader will return full command/event or return only 1 byte for wrong response bytes
	// fw v1.4.6_0012 returns 0x937162AD at start of each command/event
	// fw v1.5.0_0026 returns 0xC0 at start and at end of each command/event
	reader := bglib.NewReader(btchip)
	for {
		n, err := reader.Read(p)
		if err != nil {
			continue
		}

		if n >= 4 {
			// don't care if skip len lower than 5 bytes
			if skipN >= 5 {
				log.Warn().Hex("data", skipBuf[:skipN]).Msg("Skip wrong bytes")
			}
			skipN = 0

			header, data := bglib.DecodeResponse(p, n)

			// process only logs
			switch header {
			case bglib.Evt_le_gap_extended_scan_response:
				log.WithLevel(btgap).Hex("data", p[:n]).Msg("<=btgap")
			default:
				log.WithLevel(btraw).Hex("data", p[:n]).Msg("<=btraw")
			}

			// process data
			switch header {
			case bglib.Cmd_system_get_bt_address:
				gw = newGatewayDevice(data["mac"].(string))
			case bglib.Cmd_le_gap_set_discovery_extended_scan_response:
				log.Debug().Msg("cmd_le_gap_set_discovery_extended_scan_response")
				// no need to forward response to this command
				n = 0
			case bglib.Evt_system_boot:
				if !bglib.IsResetCmd(btchipReq) {
					// silabs_ncp_bt reboot chip at startup using GPIO
					log.Debug().Msg("Hardware chip reboot detected")
					// no need to forward event in this case
					continue
				}
			case bglib.Evt_le_gap_extended_scan_response:
				n = btchipProcessExtResponse(p[:n])
			}

			if p[0] == 0x20 || header == bglib.Evt_system_boot {
				btchipRespClear()
			}
		} else {
			skipBuf[skipN] = p[0]
			skipN++
		}

		_, err = btapp.Write(p[:n])
		if err != nil {
			log.Fatal().Err(err).Send()
		}
	}
}

// raw commands chan to BT chip, because we should send new command
// only after receive response from previous command
var btchipQueue = make(chan []byte, 100)

// raw response chan from BT chip, receive only commands responses
var btchipResp = make(chan bool)

func btchipQueueAdd(p []byte) {
	btchipQueue <- p
}

func btchipQueueClear() {
	if len(btchipQueue) > 0 {
		select {
		case <-btchipQueue:
		default:
		}
	}
}

// unblock btchipResp chan even if no waiters
func btchipRespClear() {
	select {
	case btchipResp <- true:
	default:
	}
}

var btchipReq []byte

func btchipWriter() {
	for btchipReq = range btchipQueue {
		log.WithLevel(btraw).Hex("data", btchipReq).Msg("=>btraw")

		if _, err := btchip.Write(btchipReq); err != nil {
			log.Fatal().Err(err).Send()
		}

		//log.Debug().Msg("wait")
		<-btchipResp
		//log.Debug().Msg("continue")
	}
}

var btchipRepeatFilter = RepeatFilter{cache: make(map[string]time.Time), clear: time.Now()}

// btchipProcessExtResponse unpack GAP extended scan response from BT chip
// skips same data for 5 seconds
// conveverts data to simple scan response because BT app can process only it
func btchipProcessExtResponse(data []byte) int {
	// save rssi before clear
	rssi := data[17]

	// clear rssi and channel before repeatFileter check
	data[17] = 0
	data[18] = 0

	// check if message in repeatFileter
	if btchipRepeatFilter.Test(string(data)) {
		return 0
	}

	// restore rssi
	data[17] = rssi

	n := bglib.ConvertExtendedToLegacy(data)
	msg := gap.ParseScanResponse(data[:n])

	var payload gap.Map

	switch msg.ServiceUUID {
	case 0x181A:
		payload = gap.ParseATC1441(msg.Raw[0x16][2:])
		btchipProcessBLE(msg.MAC, "atc1441", payload, true)

	case 0x181B:
		payload = gap.ParseMiScalesV2(msg.Raw[0x16][2:])
		btchipProcessBLE(msg.MAC, "miscales2", payload, false)

	case 0x181D:
		payload = gap.ParseMiScalesV1(msg.Raw[0x16][2:])
		btchipProcessBLE(msg.MAC, "miscales", payload, false)

	case 0xFE95:
		mibeacon, useful := gap.ParseMiBeacon(msg.Raw[0x16][2:], config.GetBindkey)
		//log.Debug().Uint8("useful", useful).Msgf("%+v", mibeacon)
		if useful > 0 {
			if useful == 1 {
				// is encrypted
				miioBleQueryDev(mibeacon.Mac, mibeacon.Pdid)
			}
			advType := fmt.Sprintf("mi:%d", mibeacon.Pdid)
			btchipProcessBLE(msg.MAC, advType, mibeacon.Decode(), true)
		}
	}

	switch msg.CompanyID {
	case 0x004C: // iBeacon
		if msg.Raw[0xFF][2] == 2 && msg.Raw[0xFF][3] == 0x15 {
			payload = gap.ParseIBeacon(msg.Raw[0xFF][2:])
			id := fmt.Sprintf("%s-%d-%d", payload["uuid"], payload["major"], payload["minor"])
			btchipProcessBLETracker(id, "ibeacon", msg.RSSI)
		}
	case 0x00D2: // Nut
		btchipProcessBLETracker(msg.MAC, "nut", msg.RSSI)
	case 0x0157: // MiBand or Amazfit Watch
		// don't know how to parse payload, but can be used as tracker
		btchipProcessBLETracker(msg.MAC, "miband", msg.RSSI)
	}

	return n
}

func btchipProcessBLE(mac string, advType string, data gap.Map, merge bool) {
	device, ok := devices[mac]
	if !ok {
		device = newBLEDevice(mac, advType)
	}
	device.(*BLEDevice).updateState(data, merge)
}

var btchipTrackers = make(map[string]uint8)

func btchipProcessBLETracker(mac string, advType string, rssi int8) {
	// detects tracker only after 10 events
	if _, ok := btchipTrackers[mac]; !ok {
		btchipTrackers[mac] = 1
		return
	} else if btchipTrackers[mac] < 10 {
		btchipTrackers[mac]++
		return
	}

	device, ok := devices[mac]
	if !ok {
		device = newBLEDevice(mac, advType)
	}

	// if gw init after first BLE events...
	if gw == nil {
		return
	}
	data := gap.Map{"rssi": rssi, "area": gw.WiFi.MAC}
	device.(*BLEDevice).updateState(data, false)
}

type RepeatFilter struct {
	cache map[string]time.Time
	clear time.Time
}

func (r *RepeatFilter) Test(key string) bool {
	now := time.Now()
	if r.clear.After(now) {
		for k, v := range r.cache {
			if now.After(v) {
				delete(r.cache, k)
			}
		}
		// clear cache once per minute
		r.clear = now.Add(time.Minute)
	}

	if ts, ok := r.cache[key]; ok && now.Before(ts) {
		return true
	}

	// put key in cache on 5 seconds
	r.cache[key] = now.Add(time.Second * 5)

	return false
}
