package main

type GatewayDevice struct {
	Type      string `json:"type"`
	FwVersion string `json:"fw_version,omitempty"`
	//Miio      struct {
	//	Did string `json:"did,omitempty"`
	//} `json:"miio"`
	WiFi struct {
		MAC  string `json:"mac,omitempty"`
		Addr string `json:"addr,omitempty"`
	} `json:"wifi"`
	BT struct {
		Addr      uint16 `json:"addr,omitempty"`
		FwVersion string `json:"fw_version,omitempty"`
		IVIndex   uint32 `json:"ivi"`
	} `json:"bt"`
}

func newGatewayDevice(mac string) *GatewayDevice {
	device := &GatewayDevice{Type: "gateway"}
	device.WiFi.MAC = mac
	//device.getDid()
	devices[mac] = device
	mqttPublish("gw3/"+mac+"/info", device, true)
	return device
}

func (d *GatewayDevice) updateBT(fw string, addr uint16, ivi uint32) {
	d.BT.FwVersion = fw
	d.BT.Addr = addr
	d.BT.IVIndex = ivi
	mqttPublish("gw3/"+d.WiFi.MAC+"/info", d, true)
}

//func (d *GatewayDevice) getDid() {
//	data, err := ioutil.ReadFile("/data/miio/device.conf")
//	if err != nil {
//		log.Fatalln(err)
//	}
//	d.Miio.Did = string(data[bytes.IndexByte(data, '=')+1 : bytes.IndexByte(data, '\n')])
//}
