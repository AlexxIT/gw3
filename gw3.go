/**
log.Panic().Err(err).Send() - output to default and stderr with trace and exit app
log.Fatal() - output to default and exit app, useless!
log.Error().Caller().Err(err).Send() - output to default with line number
*/
package main

import (
	"encoding/json"
	"flag"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"strings"
	"time"
)

var (
	config  = &Config{}
	devices = make(map[string]interface{})
	gw      = newGatewayDevice()
	version string
)

func main() {
	mainInitConfig()

	shellUpdatePath()

	// kill daemon_miio.sh before kill silabs_ncp_bt
	shellKillall("daemon_miio.sh")
	// kill silabs_ncp_bt before open TTY
	shellKillall("silabs_ncp_bt")

	btappInit()
	btchipInit()

	// need to restart zigbee_gw after restart mosquitto
	if shellRunMosquitto() {
		shellKillall("zigbee_gw")
	}

	// patch daemon_miio.sh if needed
	shellPatchApp("daemon_miio.sh")

	// run daemon_miio.sh what runs other apps from /tmp or /bin
	shellRunDaemon()

	go miioReader()
	go mqttReader()

	go btchipReader()
	go btappReader()

	select {} // run forever
}

func mainInitConfig() {
	v := flag.Bool("v", false, "Prints current version")

	logs := flag.String("log", "",
		"Logs modes: debug,info + btraw,btgap,miio + syslog,mqtt + json,text")

	flag.DurationVar(&config.discoveryDelay, "dd", time.Minute, "BLE discovery delay")
	flag.DurationVar(&config.patchDelay, "pd", 5*time.Minute, "Silabs patch delay, 0 - disabled")

	flag.Parse()

	if *v {
		println(version)
		os.Exit(0)
	}

	if data, err := ioutil.ReadFile("/data/gw3.json"); err == nil {
		if err = json.Unmarshal(data, config); err != nil {
			log.Panic().Err(err).Send()
		}
	}

	mainInitLogger(*logs)
}

// additional log levels for advanced output
var btraw, btgap, btskip, miioraw zerolog.Level

// log levels: debug, info, warn (default)
// advanced debug:
//   - btraw - all BT raw data except GAP
//   - btgap - only BT GAP raw data
//   - miio - miio raw data
// log out: syslog, mqtt, stdout (default)
// log format: json, text (nocolor), console (default)
func mainInitLogger(logs string) {
	if strings.Contains(logs, "debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if strings.Contains(logs, "info") {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}

	if strings.Contains(logs, "btraw") {
		btraw = zerolog.NoLevel
	} else {
		btraw = zerolog.Disabled
	}
	if strings.Contains(logs, "btgap") {
		btgap = zerolog.NoLevel
	} else {
		btgap = zerolog.Disabled
	}
	if strings.Contains(logs, "btskip") {
		btskip = zerolog.Disabled
	} else {
		btskip = zerolog.WarnLevel
	}
	if strings.Contains(logs, "miio") {
		miioraw = zerolog.NoLevel
	} else {
		miioraw = zerolog.Disabled
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	var writer io.Writer
	if strings.Contains(logs, "syslog") {
		var err error
		writer, err = syslog.New(syslog.LOG_USER|syslog.LOG_NOTICE, "gw3")
		if err != nil {
			log.Panic().Err(err).Send()
		}
	} else if strings.Contains(logs, "mqtt") {
		writer = mqttLogWriter{}
	} else {
		writer = os.Stdout
	}
	if !strings.Contains(logs, "json") {
		nocolor := writer != os.Stdout || strings.Contains(logs, "text")
		writer = zerolog.ConsoleWriter{Out: writer, TimeFormat: "15:04:05.000", NoColor: nocolor}
	}
	log.Logger = log.Output(writer)
}

type Config struct {
	Devices        map[string]ConfigDevice `json:"devices,omitempty"`
	discoveryDelay time.Duration
	patchDelay     time.Duration
}

type ConfigDevice struct {
	Bindkey string `json:"bindkey,omitempty"`
}

func (c *Config) GetBindkey(mac string) string {
	if device, ok := c.Devices[mac]; ok {
		return device.Bindkey
	}
	return ""
}

func (c *Config) SetBindKey(mac string, bindkey string) {
	if c.Devices == nil {
		c.Devices = make(map[string]ConfigDevice)
	}
	if device, ok := c.Devices[mac]; ok {
		if device.Bindkey == bindkey {
			// skip if nothing changed
			return
		}
		device.Bindkey = bindkey
	} else {
		c.Devices[mac] = ConfigDevice{Bindkey: bindkey}
	}

	data, err := json.Marshal(c)
	if err != nil {
		log.Error().Caller().Err(err).Send()
		return
	}
	log.Info().Str("mac", mac).Msg("Write new bindkey to config")

	if err = ioutil.WriteFile("/data/gw3.json", data, 0666); err != nil {
		log.Error().Caller().Err(err).Send()
	}
}

type DeviceGetSet interface {
	getState()
	setState(p []byte)
}
