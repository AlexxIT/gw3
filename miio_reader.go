package main

import (
	"github.com/AlexxIT/gw3/dict"
	"github.com/AlexxIT/gw3/miio"
	"github.com/rs/zerolog/log"
	"net"
	"time"
)

var miioConn net.Conn

var miioBleQueries = make(map[string]time.Time)

func miioReader() {
	var err error

	miioConn, err = net.Dial("unixpacket", "/tmp/miio_agent.socket")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	// bind 8 - homekitapp
	p := miio.EncodeBind(128)
	if _, err = miioConn.Write(p); err != nil {
		log.Fatal().Err(err).Send()
	}

	p = make([]byte, 1024)
	var n int

	for {
		n, err = miioConn.Read(p)
		if err != nil {
			log.Warn().Err(err).Send()
			continue
		}

		log.WithLevel(miioraw).RawJSON("data", p[:n]).Msg("<=miio")

		method := miio.DecodeMethod(p[:n])
		if method == "" {
			continue
		}

		var data *dict.Dict

		data, err = dict.Unmarshal(p[:n])
		if err != nil {
			log.Warn().Err(err).Send()
			continue
		}

		switch method {
		case miio.BleQueryDev:
			result := data.GetDict("result")
			if result != nil {
				mac := result.GetString("mac", "")
				beaconkey := result.GetString("beaconkey", "")
				config.SetBindKey(mac, beaconkey)
			}
		}
	}
}

func miioBleQueryDev(mac string, pdid uint16) {
	//	// not more than once in 30 minutes
	now := time.Now()
	if ts, ok := miioBleQueries[mac]; ok && now.Before(ts) {
		return
	}
	miioBleQueries[mac] = now.Add(time.Minute * 30)

	log.Debug().Str("mac", mac).Msg("Query bindkey")

	p := miio.EncodeBleQueryDev(mac, pdid)

	log.WithLevel(miioraw).RawJSON("data", p).Msg("=>miio")

	if _, err := miioConn.Write(p); err != nil {
		log.Warn().Err(err).Send()
	}
}
