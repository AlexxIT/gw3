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
		if _, err = os.Stat("/tmp/true_agent.socket"); os.IsNotExist(err) {
			time.Sleep(time.Second)
			continue
		}

		var conn1, conn2 net.Conn

		conn1, err = sock.Accept()
		if err != nil {
			log.Panic().Err(err).Send()
		}

		// original socket from miio_agent
		conn2, err = net.Dial("unixpacket", "/tmp/true_agent.socket")
		if err != nil {
			log.Panic().Err(err).Send()
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
			log.Debug().Uint8("addr", *addr).Msg("Close miio connection")
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
				if incoming && data.GetString("method", "") == "properties_changed" {
					if props := data.GetArrayItem("params", 0); props != nil {
						miioDecodeGatewayProps(props)
					}
				}
			}
		}

		if _, err = conn2.Write(b[:n]); err != nil {
			break
		}
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

func miioDecodeGatewayProps(props *dict.Dict) {
	if props.GetUint8("siid", 0) != 3 {
		return
	}
	switch props.GetUint8("piid", 0) {
	case 1:
		if value, ok := props.TryGetNumber("value"); ok {
			switch value {
			case 0:
				gw.updateAlarmState("disarmed")
			case 1:
				gw.updateAlarmState("armed_home")
			case 2:
				gw.updateAlarmState("armed_away")
			case 3:
				gw.updateAlarmState("armed_night")
			}
		}
	case 22:
		if value, ok := props.TryGetNumber("value"); ok {
			switch value {
			case 0:
				gw.updateAlarmState("")
			case 1:
				gw.updateAlarmState("triggered")
			}
		}
	}
}
