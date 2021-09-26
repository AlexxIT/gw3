package main

import (
	"github.com/AlexxIT/gw3/dict"
	"github.com/AlexxIT/gw3/gap"
	"github.com/rs/zerolog/log"
)

type BLEDevice struct {
	Type  string `json:"type"`
	Brand string `json:"brand"`
	Name  string `json:"name"`
	Model string `json:"model"`
	MAC   string `json:"mac"`
	state gap.Map
}

var brands = []string{
	"atc1441", "Xiaomi", "TH Sensor ATC", "ATC1441",
	"miscales", "Xiaomi", "Mi Scale", "XMTZC01HM",
	"miscales2", "Xiaomi", "Mi Scale 2", "XMTZC04HM",
	"ibeacon", "Apple", "iBeacon", "iBeacon",
	"nut", "NutFind", "Nut", "Nut",
	"miband", "Xiaomi", "Mi Band", "Mi Band",
	"mi:152", "Xiaomi", "Flower Care", "HHCCJCY01",
	"mi:131", "Xiaomi", "Kettle", "YM-K1501", // CH, HK, RU version
	"mi:275", "Xiaomi", "Kettle", "YM-K1501", // international
	"mi:339", "Yeelight", "Remote Control", "YLYK01YL",
	"mi:349", "Xiaomi", "Flower Pot", "HHCCPOT002",
	"mi:426", "Xiaomi", "TH Sensor", "LYWSDCGQ/01ZM",
	"mi:794", "Xiaomi", "Door Lock", "MJZNMS02LM",
	"mi:839", "Xiaomi", "Qingping TH Sensor", "CGG1",
	"mi:860", "Xiaomi", "Scooter M365 Pro", "DDHBC02NEB", // only tracking
	"mi:903", "Xiaomi", "ZenMeasure TH", "MHO-C401",
	"mi:950", "Yeelight", "Dimmer", "YLKG07YL",
	"mi:959", "Yeelight", "Heater Remote", "YLYB01YL-BHFRC",
	"mi:982", "Xiaomi", "Qingping Door Sensor", "CGH1",
	"mi:1034", "Xiaomi", "Mosquito Repellent", "WX08ZM",
	"mi:1115", "Xiaomi", "TH Clock", "LYWSD02MMC",
	"mi:1116", "Xiaomi", "Viomi Kettle", "V-SK152", // international
	"mi:1161", "Xiaomi", "Toothbrush T500", "MES601",
	"mi:1249", "Xiaomi", "Magic Cube", "XMMF01JQD",
	"mi:1254", "Yeelight", "Fan Remote", "YLYK01YL-VENFAN",
	"mi:1371", "Xiaomi", "TH Sensor 2", "LYWSD03MMC",
	"mi:1398", "Xiaomi", "Alarm Clock", "CGD1",
	"mi:1647", "Xiaomi", "Qingping TH Lite", "CGDK2",
	"mi:1678", "Yeelight", "Fan Remote", "YLYK01YL-FANCL",
	"mi:1694", "Aqara", "Door Lock N100", "ZNMS16LM",
	"mi:1695", "Aqara", "Door Lock N200", "ZNMS17LM",
	"mi:1747", "Xiaomi", "ZenMeasure Clock", "MHO-C303",
	"mi:1983", "Yeelight", "Button S1", "YLAI003",
	"mi:2038", "Xiaomi", "Night Light 2", "MJYD02YL-A", // 15,4103,4106,4119,4120
	"mi:2147", "Xiaomi", "Water Leak Sensor", "SJWS01LM",
	"mi:2443", "Xiaomi", "Door Sensor 2", "MCCGQ02HL",
	"mi:2444", "Xiaomi", "Door Lock", "XMZNMST02YD",
	"mi:2455", "Honeywell", "Smoke Alarm", "JTYJ-GD-03MI",
	"mi:2480", "Xiaomi", "Safe Box", "BGX-5/X1-3001",
	"mi:2691", "Xiaomi", "Qingping Motion Sensor", "CGPR1",
	"mi:2701", "Xiaomi", "Motion Sensor 2", "RTCGQ02LM", // 15,4119,4120
	"mi:2888", "Xiaomi", "Qingping TH Sensor", "CGG1", // same model as 839?!
}

func newBLEDevice(mac string, advType string) *BLEDevice {
	device := &BLEDevice{
		Type: "ble",
		MAC:  mac,
	}

	for i := 0; i < len(brands); i += 4 {
		if advType == brands[i] {
			device.Brand = brands[i+1]
			device.Name = brands[i+2]
			device.Model = brands[i+3]
			break
		}
	}

	devices[mac] = device
	mqttPublish("gw3/"+mac+"/info", device, true)
	return device
}

func (d *BLEDevice) updateState(data gap.Map) {
	if data.IsEvent() {
		mqttPublish("gw3/"+d.MAC+"/event", data, false)
		return
	}

	if d.state != nil {
		for k, v := range data {
			d.state[k] = v
		}
	} else {
		d.state = data
	}
	mqttPublish("gw3/"+d.MAC+"/state", d.state, true)
}

func (d *BLEDevice) getState() {
	// BLE device can't get state
}

func (d *BLEDevice) setState(p []byte) {
	payload, err := dict.Unmarshal(p)
	if err != nil {
		log.Warn().Err(err).Send()
		return
	}
	if value, ok := payload.TryGetString("bindkey"); ok {
		config.SetBindKey(d.MAC, value)
	}
}
