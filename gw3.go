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
)

func main() {
	mainInitLogger()
	mainInitConfig()

	shellUpdatePath()
	shellDaemonStop()
	shellSilabsStop()
	shellFreeTTY()

	btappInit()
	btchipInit()

	shellDaemonStart()

	go btchipReader()
	go btchipWriter()

	go btappReader()
	go mqttReader()
	go miioReader()

	select {} // run forever
}

var (
	// additional log levels for advanced output
	btraw   = zerolog.Disabled
	btgap   = zerolog.Disabled
	miioraw = zerolog.Disabled
)

func mainInitLogger() {
	// log levels: debug, info, warn (default)
	// advanced debug:
	//   - btraw - all BT raw data except GAP
	//   - btgap - only BT GAP raw data
	//   - miio - miio raw data
	// log out: syslog, mqtt, stdout (default)
	// log format: json, text (nocolor), console (default)
	logs := flag.String("log", "warn+stdout+console",
		"Logs modes: debug,info + btraw,btgap,miio + syslog,mqtt + json,text")
	flag.DurationVar(&config.discoveryDelay, "dd", time.Minute, "BLE discovery delay")
	flag.DurationVar(&config.patchDelay, "pd", 5*time.Minute, "Silabs patch delay, 0 - disabled")

	flag.Parse()

	if strings.Contains(*logs, "debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if strings.Contains(*logs, "info") {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}

	if strings.Contains(*logs, "btraw") {
		btraw = zerolog.NoLevel
	}
	if strings.Contains(*logs, "btgap") {
		btgap = zerolog.NoLevel
	}
	if strings.Contains(*logs, "miio") {
		miioraw = zerolog.NoLevel
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	var writer io.Writer
	if strings.Contains(*logs, "syslog") {
		var err error
		writer, err = syslog.New(syslog.LOG_USER|syslog.LOG_NOTICE, "gw3")
		if err != nil {
			log.Fatal().Err(err).Send()
		}
	} else if strings.Contains(*logs, "mqtt") {
		writer = mqttLogWriter{}
	} else {
		writer = os.Stdout
	}
	if !strings.Contains(*logs, "json") {
		nocolor := writer != os.Stdout || strings.Contains(*logs, "text")
		writer = zerolog.ConsoleWriter{Out: writer, TimeFormat: "15:04:05.000", NoColor: nocolor}
	}
	log.Logger = log.Output(writer)
}

func mainInitConfig() {
	data, err := ioutil.ReadFile("/data/gw3.json")
	if err != nil {
		return
	}
	if err = json.Unmarshal(data, config); err != nil {
		log.Fatal().Err(err).Send()
	}
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
	c.Devices[mac] = ConfigDevice{Bindkey: bindkey}

	data, err := json.Marshal(c)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	log.Info().Str("mac", mac).Msg("Write new bindkey to config")

	if err = ioutil.WriteFile("/data/gw3.json", data, 0666); err != nil {
		log.Fatal().Err(err).Send()
	}
}

type DeviceGetSet interface {
	getState()
	setState(p []byte)
}
