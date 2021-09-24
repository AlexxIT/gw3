package main

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"os/exec"
	"time"
)

func shellSilabsStop() {
	log.Debug().Msg("Kill daemon_miio.sh and silabs_ncp_bt")
	_ = exec.Command("killall", "daemon_miio.sh").Run()
	_ = exec.Command("killall", "silabs_ncp_bt").Run()
}

func shellSilabsStart() {
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

	log.Debug().Msg("Run daemon_miio.sh and silabs_ncp_bt")
	// run patched script without error processing
	_ = exec.Command("sh", "-c", "/tmp/daemon_miio.sh&").Start()
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
	data = bytes.Replace(data, []byte("/data/miio/mible_local.db"), []byte("/var/tmp/mible_local.db\x00\x00"), 1)
	data = bytes.Replace(data, []byte("/data/miio/sbt_record_db"), []byte("/var/tmp/sbt_record_db\x00\x00"), 1)

	if err = ioutil.WriteFile("/var/tmp/silabs_ncp_bt", data, 0x777); err != nil {
		log.Fatal().Err(err).Send()
	}

	// copy databases
	_ = exec.Command("cp", "/data/miio/mible_local.db", "/data/miio/mible_local.db-shm",
		"/data/miio/mible_local.db-wal", "/data/miio/sbt_record_db", "/var/tmp/").Run()
}

func shellSilabsWatchdog() *time.Timer {
	t := time.NewTimer(time.Minute)
	go func() {
		for {
			<-t.C
			log.Warn().Msg("Restart silabs_ncp_bt after timeout")
			_ = exec.Command("killall", "silabs_ncp_bt").Run()
			t.Reset(time.Minute)
		}
	}()
	return t
}