package main

import (
	proto "github.com/huin/mqtt"
	"github.com/jeffallen/mqtt"
	log "github.com/sirupsen/logrus"
	"gw3/miio"
	"os/exec"
	"time"
)

var (
	config      *Config
	miioClient  *miio.Miio
	mqttClient  *mqtt.ClientConn
	mqttPayload *proto.Publish
)

func main() {
	config = initConfig()

	if config.Mqtt.Host != "" {
		initMQTT()
	}

	if config.silabs {
		log.Infoln("Kill daemon_miio.sh and silabs_ncp_bt")
		_ = exec.Command("killall", "daemon_miio.sh", "silabs_ncp_bt").Run()
	}

	uart, silabs := initSerials()

	if config.silabs {
		patchSilabs()

		go func() {
			// TODO: restart silabs_ncp_bt
			log.Infoln("Run: /tmp/silabs_ncp_bt /dev/ttyp8 1")
			_ = exec.Command("/tmp/silabs_ncp_bt", "/dev/ttyp8", "1").Run()
			// exit from silabs
			log.Fatalln("silabs_ncp_bt exit")
		}()
		go func() {
			time.Sleep(time.Second * 5)
			log.Infoln("Run: daemon_miio.sh")
			_ = exec.Command("daemon_miio.sh&").Start()
		}()
	}

	go silabs2uart(silabs, uart)
	go uart2silabs(uart, silabs)

	miioHandler := func(mac string, bindkey string) {
		log.Debugln("Get bindkey for", mac)
		config.AddDevice(mac, bindkey)
	}
	miioClient = miio.NewClient()
	miioClient.Start(miioHandler)
}
