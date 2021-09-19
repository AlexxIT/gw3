package main

import (
	"encoding/json"
	"flag"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

var (
	config  = &Config{}
	devices = make(map[string]interface{})
	gw      *GatewayDevice
)

func main() {
	mainInitLogger()
	mainInitConfig()

	silabsStop()

	btappInit()
	btchipInit()

	silabsStart()

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
	//mqttraw = zerolog.Disabled
	miioraw = zerolog.Disabled
)

func mainInitLogger() {
	logs := flag.String("log", "", "sets log level to debug")
	logfile := flag.String("logfile", "", "output log into file instead of stdout")

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
	//if strings.Contains(*logs, "mqtt") {
	//	mqttraw = zerolog.NoLevel
	//}
	if strings.Contains(*logs, "miio") {
		miioraw = zerolog.NoLevel
	}

	if strings.Contains(*logs, "file") {
		miioraw = zerolog.NoLevel
	}

	var writer io.Writer
	if *logfile != "" {
		writer, _ = os.Create(*logfile)
	} else {
		writer = os.Stderr
	}
	writer = zerolog.ConsoleWriter{Out: writer, TimeFormat: "15:04:05.000"}
	//writer := zerolog.MultiLevelWriter(writer, mqttLogWriter{})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
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
