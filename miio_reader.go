package main

import (
	"fmt"
	"github.com/AlexxIT/gw3/dict"
	"github.com/rs/zerolog/log"
	"net"
	"os"
	"time"
)

func miioReader() {
	_ = os.Remove("/tmp/miio_agent.socket")

	sock, err := net.Listen("unixpacket", "/tmp/miio_agent.socket")
	if err != nil {
		log.Panic().Err(err).Send()
	}

	for {
		var conn1, conn2 net.Conn

		if conn1, err = sock.Accept(); err != nil {
			log.Panic().Err(err).Send()
		}

		// original socket from miio_agent
		for {
			if conn2, err = net.Dial("unixpacket", "/tmp/true_agent.socket"); err == nil {
				break
			}
			time.Sleep(time.Second)
		}

		var addr uint8
		go miioSocketProxy(conn1, conn2, true, &addr)
		go miioSocketProxy(conn2, conn1, false, &addr)
	}
}

const (
	Basic      = uint8(0b1)
	Bluetooth  = uint8(0b10)
	Zigbee     = uint8(0b100)
	HomeKit    = uint8(0b1000)
	Automation = uint8(0b10000)
	Gateway    = uint8(0b100000)
)

var miioConn = make(map[uint8]miioPair)

type miioPair struct {
	inc net.Conn
	out net.Conn
}

func miioSocketProxy(conn1, conn2 net.Conn, incoming bool, addr *uint8) {
	var data *dict.Dict

	var msg string
	if incoming {
		msg = "miio<-"
	} else {
		msg = "<-miio"
	}

	var fixZigbee3Bind uint32

	var b = make([]byte, 1024)
	for {
		n, err := conn1.Read(b)
		if err != nil {
			break
		}

		log.WithLevel(miioraw).Uint8("addr", *addr).RawJSON("data", b[:n]).Msg(msg)

		if data, err = dict.Unmarshal(b[:n]); err == nil {
			switch *addr {
			case 0:
				if incoming && data.GetString("method", "") == "bind" {
					*addr = data.GetUint8("address", 0)
					miioConn[*addr] = miioPair{inc: conn1, out: conn2}

					log.Debug().Uint8("addr", *addr).Msg("Open miio connection")
				}
			case Bluetooth:
				if !incoming {
					if result := data.GetDict("result"); result != nil {
						if result.GetString("operation", "") == "query_dev" {
							mac := result.GetString("mac", "")
							bindkey := result.GetString("beaconkey", "")
							config.SetBindKey(mac, bindkey)
						}
					}
				}
			case Zigbee:
				if incoming {
					switch data.GetString("method", "") {
					case "event.gw.heartbeat":
						if param := data.GetArrayItem("params", 0); param != nil {
							// {"free_mem":5600,"ip":"192.168.1.123","load_avg":"3.18|3.05|2.79|3/95|25132","rssi":65,
							//  "run_time":43783,"setupcode":"123-45-678","ssid":"WiFi","tz":"GMT3"}
							(*param)["action"] = "heartbeat"
							gw.updateEvent(param)
						}
					case "_sync.zigbee3_bind":
						fixZigbee3Bind = data.GetUint32("id", 0)
					}
				} else {
					if fixZigbee3Bind > 0 && data.GetUint32("id", 0) == fixZigbee3Bind {
						log.Info().Msg("Patch zigbee3_bind command")

						s := fmt.Sprintf(`{"result":{"code":0,"message":"ok"},"id":%d}`, fixZigbee3Bind)
						n = copy(b, s)

						fixZigbee3Bind = 0
					}
				}
			case Gateway:
				switch data.GetString("method", "") {
				case "properties_changed":
					if props := data.GetArrayItem("params", 0); props != nil {
						miioDecodeGatewayProps(props)
					}
				case "local.report":
					if params := data.GetDict("params"); params != nil {
						if button, ok := params.TryGetString("button"); ok {
							// click and double_click
							data = &dict.Dict{"action": button}
							gw.updateEvent(data)
						}
					}
				}
			}
		}

		if _, err = conn2.Write(b[:n]); err != nil {
			break
		}
	}

	if incoming {
		log.Debug().Uint8("addr", *addr).Msg("Close miio connection")
	}

	_ = conn2.Close()
	if *addr > 0 {
		delete(miioConn, *addr)
	}
}

var miioBleQueries = make(map[string]time.Time)

func miioBleQueryDev(mac string, pdid uint16) {
	pair, ok := miioConn[Bluetooth]
	if !ok {
		log.Debug().Msg("Can't query bindkey")
		return
	}

	var ts time.Time

	// not more than once in 60 minutes
	now := time.Now()
	if ts, ok = miioBleQueries[mac]; ok && now.Before(ts) {
		return
	}
	miioBleQueries[mac] = now.Add(time.Hour)

	log.Debug().Str("mac", mac).Msg("Query bindkey")

	id := uint32(time.Now().Nanosecond()) & 0xFFFFFF
	p := []byte(fmt.Sprintf(
		`{"id":%d,"method":"_sync.ble_query_dev","params":{"mac":"%s","pdid":%d}}`,
		id, mac, pdid,
	))

	if _, err := pair.out.Write(p); err != nil {
		log.Warn().Err(err).Send()
	}
}

// name: {siid, piid, value}
var miioAlarmStates = map[string][3]uint8{
	"disarmed":    {3, 1, 0},
	"armed_home":  {3, 1, 1},
	"armed_away":  {3, 1, 2},
	"armed_night": {3, 1, 3},
	"":            {3, 22, 0},
	"triggered":   {3, 22, 1},
}

func miioEncodeGatewayProps(state string) {
	pair, ok := miioConn[Gateway]
	if !ok {
		log.Debug().Msg("Can't set gateway props")
		return
	}

	if v, ok := miioAlarmStates[state]; ok {
		id := uint32(time.Now().Nanosecond()) & 0xFFFFFF
		p := []byte(fmt.Sprintf(
			`{"from":"4","id":%d,"method":"set_properties","params":[{"did":"%s","piid":%d,"siid":%d,"value":%d}]}`,
			id, gw.Miio.Did, v[1], v[0], v[2],
		))

		if _, err := pair.inc.Write(p); err != nil {
			log.Warn().Err(err).Send()
		}
	}
}

func miioDecodeGatewayProps(props *dict.Dict) {
	siid := props.GetUint8("siid", 0)
	piid := props.GetUint8("piid", 0)
	value := props.GetUint8("value", 255)
	for k, v := range miioAlarmStates {
		if v[0] == siid && v[1] == piid && v[2] == value {
			gw.updateAlarmState(k)
			return
		}
	}
}

func miioEncodeGatewayBuzzer(duration uint64, volume uint8) {
	pair, ok := miioConn[Basic]
	if !ok {
		log.Debug().Msg("Can't run buzzer")
		return
	}

	var b []byte
	id := uint32(time.Now().Nanosecond()) & 0xFFFFFF

	if volume > 0 {
		b = []byte(fmt.Sprintf(
			`{"from":32,"id":%d,"method":"local.status","params":"start_alarm,%d,%d"}`,
			id, duration, volume,
		))
	} else {
		b = []byte(fmt.Sprintf(
			`{"from":32,"id":%d,"method":"local.status","params":"stop_alarm"}`, id,
		))
	}

	if _, err := pair.inc.Write(b); err != nil {
		log.Warn().Err(err).Send()
	}
}
