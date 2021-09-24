package main

import (
	"encoding/json"
	"flag"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"strings"
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
	logs := flag.String("log", "", "values: trace,debug,info,btraw,btgap,miio")

	flag.Parse()

	if strings.Contains(*logs, "trace") {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else if strings.Contains(*logs, "debug") {
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
	writer := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"}
	if strings.Contains(*logs, "mqtt") {
		log.Logger = log.Output(zerolog.MultiLevelWriter(writer, mqttLogWriter{}))
	} else {
		log.Logger = log.Output(writer)
	}
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
	Devices map[string]ConfigDevice `json:"devices,omitempty"`
}

type ConfigDevice struct {
	Bindkey string `json:"bindkey,omitempty"`
}

func (c *Config) GetBindkey(mac string) string {
	for k, v := range c.Devices {
		if k == mac {
			return v.Bindkey
		}
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
