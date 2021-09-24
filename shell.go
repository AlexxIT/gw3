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
)

func shellDaemonStop() {
	_ = exec.Command("killall", "daemon_miio.sh").Run()
}

func shellSilabsStop() {
	_ = exec.Command("killall", "silabs_ncp_bt").Run()
}

func shellUpdatePath() {
	_ = os.Setenv("PATH", "/tmp:"+os.Getenv("PATH"))
}

func shellDaemonStart() {
	if _, err := os.Stat("/tmp/daemon_miio.sh"); os.IsNotExist(err) {
		log.Info().Msg("Patch daemon_miio.sh")

		var data []byte
		// read original file (firmware v1.4.7_0063+)
		data, err = ioutil.ReadFile("/bin/daemon_miio.sh")
		if err != nil {
			data, err = ioutil.ReadFile("/usr/app/bin/daemon_miio.sh")
			if err != nil {
				log.Fatal().Err(err).Send()
			}
		}

		data = bytes.Replace(data, []byte("ttyS1"), []byte("ttyp8"), 1)

		// write patched script
		if err = ioutil.WriteFile("/tmp/daemon_miio.sh", data, 0x777); err != nil {
			log.Fatal().Err(err).Send()
		}
	}

	log.Debug().Msg("Run daemon_miio.sh")
	// run patched script without error processing
	_ = exec.Command("sh", "-c", "daemon_miio.sh&").Start()
}

func shellFreeTTY() {
	out, err := exec.Command("sh", "-c", "lsof | grep ptyp8 | cut -f 1").Output()
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	if len(out) == 0 {
		return
	}
	// remove leading new line: "1234\n"
	pid := string(out[:len(out)-1])
	log.Debug().Str("pid", pid).Msg("Releasing the TTY")
	_ = exec.Command("kill", pid).Run()
}

func shellDeviceInfo() (did string, mac string) {
	// did=123456789
	// key=xxxxxxxxxxxxxxxx
	// mac=54:EF:44:FF:FF:FF
	// vendor=lumi
	// model=lumi.gateway.mgl03
	data, err := ioutil.ReadFile("/data/miio/device.conf")
	if err != nil {
		log.Fatal().Err(err).Send()
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

// Zigbee and Bluetooth data is broken when writing to NAND. So we moving sqlite database to memory (tmp).
// It's not a problem to lose this base, because the gateway will restore it from the cloud.
func shellPatchSilabs() {
	if _, err := os.Stat("/tmp/silabs_ncp_bt"); !os.IsNotExist(err) {
		return
	}

	log.Info().Msg("Patch silabs_ncp_bt")

	data, err := ioutil.ReadFile("/bin/silabs_ncp_bt")
	if err != nil {
		if data, err = ioutil.ReadFile("/usr/app/bin/silabs_ncp_bt"); err != nil {
			log.Fatal().Err(err).Send()
		}
	}

	// same length before and after
	data = bytes.Replace(data, []byte("/data/"), []byte("/tmp//"), -1)
	if err = ioutil.WriteFile("/tmp/silabs_ncp_bt", data, 0x777); err != nil {
		log.Fatal().Err(err).Send()
	}

	// copy databases
	_ = exec.Command("cp", "-R", "/data/miio", "/tmp").Run()
}
