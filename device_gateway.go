package main

import (
	"errors"
	"github.com/AlexxIT/gw3/dict"
	"github.com/rs/zerolog/log"
)

type GatewayDevice struct {
	Type      string `json:"type"`
	FwVersion string `json:"fw_version,omitempty"`
	Gw3       struct {
		Version string `json:"version,omitempty"`
	} `json:"gw3"`
	Miio struct {
		Did string `json:"did,omitempty"`
	} `json:"miio"`
	WiFi struct {
		MAC  string `json:"mac,omitempty"`
		Addr string `json:"addr,omitempty"`
	} `json:"wifi"`
	BT struct {
		Addr      uint16 `json:"addr,omitempty"`
		FwVersion string `json:"fw_version,omitempty"`
		IVIndex   uint32 `json:"ivi"`
	} `json:"bt"`
	state      dict.Dict
	alarmState string
}

func newGatewayDevice() *GatewayDevice {
	did, mac := shellDeviceInfo()

	device := &GatewayDevice{Type: "gateway", state: dict.Dict{}}
	device.Gw3.Version = version
	device.Miio.Did = did
	device.WiFi.MAC = mac
	devices[mac] = device
	mqttPublish("gw3/"+mac+"/info", device, true)
	return device
}

func (d *GatewayDevice) updateInfo() {
	mqttPublish("gw3/"+d.WiFi.MAC+"/info", d, true)
}

func (d *GatewayDevice) updateState(state string) {
	// skip same state
	if d.state["state"] == state {
		return
	}
	d.state["state"] = state
	mqttPublish("gw3/"+d.WiFi.MAC+"/state", d.state, true)
}

func (d *GatewayDevice) updateAlarmState(state string) {
	if state != "triggered" {
		if state == "" {
			// restore state after triggered
			state = d.alarmState
		} else {
			// remember state before triggered
			d.alarmState = state
		}
	}
	d.state["alarm_state"] = state
	mqttPublish("gw3/"+d.WiFi.MAC+"/state", d.state, true)
}

func (d *GatewayDevice) updateEvent(data *dict.Dict) {
	mqttPublish("gw3/"+d.WiFi.MAC+"/event", data, false)
}

func (d *GatewayDevice) updateBT(fw string, addr uint16, ivi uint32) {
	d.BT.FwVersion = fw
	d.BT.Addr = addr
	d.BT.IVIndex = ivi
	mqttPublish("gw3/"+d.WiFi.MAC+"/info", d, true)
}

func (d *GatewayDevice) getState() {
	// BLE device can't get state
}

func (d *GatewayDevice) setState(p []byte) {
	payload, err := dict.Unmarshal(p)
	if err != nil {
		log.Warn().Err(err).Send()
		return
	}

	if value, ok := payload.TryGetString("alarm_state"); ok {
		miioEncodeGatewayProps(value)
	}

	if value, ok := payload.TryGetString("buzzer"); ok {
		switch value {
		case "ON":
			duration := payload.GetUint64("duration", 1)
			volume := payload.GetUint8("volume", 3)
			miioEncodeGatewayBuzzer(duration, volume)
		case "OFF":
			miioEncodeGatewayBuzzer(0, 0)
		}
	}

	if value, ok := payload.TryGetString("log"); ok {
		mainInitLogger(value)
	}

	if value, ok := payload.TryGetString("test"); ok {
		switch value {
		case "error":
			// raise unhandled error
			devices["test"].(DeviceGetSet).getState()
		case "fatal":
			err = errors.New("test")
			log.Fatal().Caller().Err(err).Send()
		case "panic":
			err = errors.New("test")
			log.Panic().Err(err).Send()
		}
	}
}
