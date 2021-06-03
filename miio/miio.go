package miio

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"time"
)

type Miio struct {
	conn    net.Conn
	queries map[string]time.Time
	id      int
}

type Response struct {
	Id     int `json:"id"`
	Result struct {
		Operation string `json:"operation"`
		Mac       string `json:"mac"`
		Beaconkey string `json:"beaconkey"`
	} `json:"result"`
}

func NewClient() *Miio {
	conn, err := net.Dial("unixpacket", "/tmp/miio_agent.socket")
	if err != nil {
		panic(err)
	}
	_, err = conn.Write([]byte("{\"method\":\"bind\",\"address\":128}\n"))
	if err != nil {
		log.Fatalln(err)
	}

	return &Miio{conn: conn, queries: make(map[string]time.Time)}
}

func (self *Miio) Start(handler func(string, string)) {
	buf := make([]byte, 1024)
	for {
		n, err := self.conn.Read(buf)
		if err != nil {
			log.Fatalln(err)
		}

		log.Debugln("Miio:", string(buf[:n]))

		var resp Response
		err = json.Unmarshal(buf[:n], &resp)
		if err != nil {
			log.Warnln(err)
			continue
		}
		if resp.Result.Operation == "query_dev" {
			handler(resp.Result.Mac, resp.Result.Beaconkey)
		}
	}
}

func (self *Miio) BleQueryDev(mac string, pdid uint16) {
	// not more than once in 30 minutes
	now := time.Now()
	if ts, ok := self.queries[mac]; ok && now.Before(ts) {
		return
	}
	self.queries[mac] = now.Add(time.Minute * 30)

	log.Debugln("Query bindkey for", mac)

	self.id += 1
	payload := fmt.Sprintf(`{"method":"_sync.ble_query_dev","params":{"mac":"%s","pdid":%d},"id":%d}`,
		mac, pdid, self.id)

	_, err := self.conn.Write([]byte(payload))
	if err != nil {
		log.Warnln(err)
	}
}
