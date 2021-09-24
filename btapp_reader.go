package main

import (
	"github.com/AlexxIT/gw3/bglib"
	"github.com/AlexxIT/gw3/serial"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"time"
)

var btapp io.ReadWriteCloser

// btappInit open serial connection to /dev/ptyp8 (virtual serial interface)
func btappInit() {
	var err error
	btapp, err = serial.Open(serial.OpenOptions{
		PortName:        "/dev/ptyp8",
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	})
	if err != nil {
		log.Fatal().Err(err).Send()
	}
}

func btappReader() {
	// only one msg per 10 seconds
	sampler := log.Sample(&zerolog.BurstSampler{
		Burst:  1,
		Period: 10 * time.Second,
	})

	var p = make([]byte, 1024)
	for {
		n, err := btapp.Read(p)
		if err != nil {
			sampler.Debug().Err(err).Msg("btapp.Read")
			continue
		}

		//log.WithLevel(btraw).Hex("data", p[:n]).Msg("=>queue")

		header := uint32(p[0])<<24 | uint32(p[2])<<8 | uint32(p[3])
		switch header {
		case bglib.Cmd_system_reset:
			log.Info().Msg("=>cmd_system_reset")

			btchipQueueClear()
			btchipRespClear()

		case bglib.Cmd_le_gap_set_discovery_timing:
			//log.Debug().Int("scan_interval", 0x10).Msg("cmd_le_gap_set_discovery_timing")

			//bglib.PatchGapDiscoveryTiming(p, 0x10, 0x10)

		case bglib.Cmd_le_gap_start_discovery:
			//log.Info().Uint8("mode", p[5]).Msg("cmd_le_gap_start_discovery")

			// enable extended scan before start cmd
			btchipQueueAdd(bglib.EncodeGapExtendedScan(1))

		case bglib.Cmd_mesh_node_set_ivrecovery_mode:
			log.Info().Uint8("enable", p[4]).Msg("=>cmd_mesh_node_set_ivrecovery_mode")
		}

		btchipQueueAdd(p[:n])
	}
}
