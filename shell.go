/*
/bin/mijia_automation  /data/mijia_automation/db.unqlite
/bin/mijia_automation  /data/mijia_automation
/bin/silabs_ncp_bt     /data/miio/mible_local.db
/bin/silabs_ncp_bt     /data/miio/mible_local.db-wal
/bin/silabs_ncp_bt     /data/miio/mible_local.db-shm
/bin/silabs_ncp_bt     /data/ble_info
/bin/miio_client       /data/miioconfig.db
/bin/miio_client       /data/miioconfig.db_unqlite_journal
/bin/basic_app         /data/basic_app/gw_devices.data
/bin/zigbee_agent      /data/zigbee/coordinator.info
/bin/zigbee_gw         /data/zigbee_gw
/bin/zigbee_gw         /data/zigbee_gw/device_properties.json
*/
package main

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

func shellKillall(filename string) {
	_ = exec.Command("killall", filename).Run()
}

func shellFreeTTY() {
	out, err := exec.Command("sh", "-c", "lsof | grep ptyp8 | cut -f 1").Output()
	if err != nil {
		log.Panic().Err(err).Send()
	}
	if len(out) == 0 {
		return
	}

	// remove leading new line: "1234\n"
	pid := string(out[:len(out)-1])
	log.Debug().Str("pid", pid).Msg("Releasing the TTY")
	_ = exec.Command("kill", pid).Run()
}

func shellUpdatePath() {
	_ = os.Setenv("PATH", "/tmp:"+os.Getenv("PATH"))
}

func shellRunDaemon() {
	log.Debug().Msg("Run daemon_miio.sh")
	// run patched script without error processing
	_ = exec.Command("sh", "-c", "daemon_miio.sh&").Run()
}

func shellRunMosquitto() bool {
	if out, err := exec.Command("sh", "-c", "ps | grep mosquitto").Output(); err == nil {
		if bytes.Contains(out, []byte("mosquitto -d")) {
			return false
		}
	} else {
		log.Fatal().Err(err).Send()
	}

	log.Debug().Msg("Run public mosquitto")

	shellKillall("mosquitto")

	time.Sleep(time.Second)

	_ = exec.Command("mosquitto", "-d").Run()

	return true
}

func shellDeviceInfo() (did string, mac string) {
	// did=123456789
	// key=xxxxxxxxxxxxxxxx
	// mac=54:EF:44:FF:FF:FF
	// vendor=lumi
	// model=lumi.gateway.mgl03
	data, err := ioutil.ReadFile("/data/miio/device.conf")
	if err != nil {
		log.Panic().Err(err).Send()
	}
	for _, line := range strings.Split(string(data), "\n") {
		if len(line) < 5 {
			continue
		}
		switch line[:3] {
		case "did":
			did = line[4:]
		case "mac":
			mac = line[4:]
		}
	}
	return
}

var shellPatchTimer *time.Timer

func shellPatchTimerStart() {
	if config.patchDelay == 0 {
		return
	}
	if _, err := os.Stat("/tmp/silabs_ncp_bt"); !os.IsNotExist(err) {
		return
	}
	if shellPatchTimer == nil {
		log.Debug().Msg("Start patch timer")
		shellPatchTimer = time.AfterFunc(config.patchDelay, func() {
			shellPatchApp("silabs_ncp_bt")
			// we need to restart daemon because new binary in tmp path
			shellKillall("daemon_miio.sh")
			shellKillall("silabs_ncp_bt")
			shellRunDaemon()
			shellPatchTimer = nil
		})
	} else {
		log.Debug().Msg("Reset patch timer")
		shellPatchTimer.Reset(config.patchDelay)
	}
}

func shellPatchTimerStop() {
	if shellPatchTimer != nil {
		log.Debug().Msg("Stop patch timer")
		shellPatchTimer.Stop()
	}
}

func shellPatchApp(filename string) bool {
	if _, err := os.Stat("/tmp/" + filename); !os.IsNotExist(err) {
		return false
	}

	log.Info().Str("file", filename).Msg("Patch app")

	// read original file (firmware v1.4.7_0063+)
	data, err := ioutil.ReadFile("/bin/" + filename)
	if err != nil {
		data, err = ioutil.ReadFile("/usr/app/bin/" + filename)
		if err != nil {
			log.Panic().Err(err).Send()
		}
	}

	switch filename {
	case "daemon_miio.sh":
		// silabs_ncp_bt will work with out proxy-TTY
		data = bytes.Replace(data, []byte("ttyS1"), []byte("ttyp8"), 1)
		// old fimware v1.4.6
		data = bytes.Replace(data, []byte("$APP_PATH/"), []byte(""), -1)
	case "silabs_ncp_bt":
		// Zigbee and Bluetooth data is broken when writing to NAND. So we moving sqlite database to memory (tmp).
		// It's not a problem to lose this base, because the gateway will restore it from the cloud.
		data = bytes.Replace(data, []byte("/data/"), []byte("/tmp//"), -1)

		// copy databases
		_ = exec.Command("cp", "-R", "/data/miio", "/data/ble_info", "/tmp/").Run()
	case "miio_agent":
		// miio_agent will work with out proxy-socket
		data = bytes.Replace(data, []byte("/tmp/miio_agent.socket"), []byte("/tmp/true_agent.socket"), -1)
	}

	// write patched script
	if err = ioutil.WriteFile("/tmp/"+filename, data, 0x777); err != nil {
		log.Panic().Err(err).Send()
	}

	return true
}
