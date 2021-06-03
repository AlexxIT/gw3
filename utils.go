package main

import (
	"bytes"
	"encoding/json"
	"flag"
	proto "github.com/huin/mqtt"
	"github.com/jeffallen/mqtt"
	log "github.com/sirupsen/logrus"
	"gw3/gap"
	"gw3/serial"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
)

type Config struct {
	autoSilabs bool
	filter     int
	Mqtt       struct {
		Host string `json:"host,omitempty"`
		User string `json:"user,omitempty"`
		Pass string `json:"pass,omitempty"`
	} `json:"mqtt,omitempty"`
	Devices map[string]Device `json:"devices,omitempty"`
	// TODO: fixme
	PdidMap map[int]int `json:"pdid_map,omitempty"`

	keys gap.Bindkeys
}

type Device struct {
	Bindkey string `json:"bindkey,omitempty"`
}

func (self *Config) Bindkeys() gap.Bindkeys {
	if self.keys == nil {
		self.keys = make(gap.Bindkeys)
		for k, v := range self.Devices {
			if v.Bindkey != "" {
				self.keys[k] = v.Bindkey
			}
		}
	}
	return self.keys
}

func (self *Config) AddDevice(mac string, bindkey string) {
	if self.Devices == nil {
		self.Devices = make(map[string]Device)
	}
	self.Devices[mac] = Device{Bindkey: bindkey}
	self.keys[mac] = bindkey

	data, err := json.Marshal(self)
	if err != nil {
		log.Fatalln(err)
	}
	log.Infoln("Write new bindkey to config")
	err = ioutil.WriteFile("/data/gw3.json", data, 0666)
	if err != nil {
		log.Fatalln(err)
	}
}

func initConfig() *Config {
	var conf Config

	data, err := ioutil.ReadFile("/data/gw3.json")
	if err == nil {
		err = json.Unmarshal(data, &conf)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		conf = Config{}
		conf.Mqtt.Host = "localhost:1883"
	}

	flag.BoolVar(&conf.autoSilabs, "autosilabs", true, "Auto handle silabs_ncp_bt")
	flag.IntVar(&conf.filter, "filter", 2, "Filter BLE messages: 0 All, 1 Interesting, 2 Useful")
	ll := flag.Int("log", 3, "Log level: 1 Fatal, 3 Warning, 4 Info, 5 Debug, 6 Trace")

	flag.Parse()

	log.SetLevel(log.Level(*ll))

	return &conf
}

func initMQTT() {
	conn, err := net.Dial("tcp", config.Mqtt.Host)
	if err != nil {
		log.Fatalln(err)
	}
	mqttClient = mqtt.NewClientConn(conn)
	mqttClient.ClientId = "gw3"
	err = mqttClient.Connect(config.Mqtt.User, config.Mqtt.Pass)
	if err != nil {
		log.Fatalln(err)
	}
	mqttPayload = &proto.Publish{
		Header:    proto.Header{},
		TopicName: "gw3/raw",
	}
}

func initSerials() (uart io.ReadWriteCloser, silabs io.ReadWriteCloser) {
	uart, err := serial.Open(serial.OpenOptions{
		PortName:        "/dev/ttyS1",
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 3,
		//RTSCTSFlowControl: true,
	})
	if err != nil {
		log.Fatalln(err)
	}

	silabs, err = serial.Open(serial.OpenOptions{
		PortName:        "/dev/ptyp8",
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	})
	if err != nil {
		log.Fatalln(err)
	}

	return
}

// Zigbee and Bluetooth data is broken when writing to NAND. So we moving sqlite database to memory (tmp).
// It's not a problem to lose this base, because the gateway will restore it from the cloud.
func patchSilabs() {
	if _, err := os.Stat("/tmp/silabs_ncp_bt"); !os.IsNotExist(err) {
		return
	}

	log.Infoln("Patch silabs_ncp_bt")

	data, err := ioutil.ReadFile("/bin/silabs_ncp_bt")
	if err != nil {
		log.Fatalln(err)
	}

	// same length before and after
	data = bytes.Replace(data, []byte("/data/miio/mible_loc"), []byte("/tmp/mible_local.db\x00"), 1)

	err = ioutil.WriteFile("/tmp/silabs_ncp_bt", data, 0x777)
	if err != nil {
		log.Fatalln(err)
	}

	// copy database
	_ = exec.Command("cp", "/data/miio/mible_local.db*", "/tmp/").Run()
}
