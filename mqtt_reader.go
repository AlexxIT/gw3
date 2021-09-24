package main

import (
	"bytes"
	"encoding/json"
	proto "github.com/huin/mqtt"
	"github.com/jeffallen/mqtt"
	"github.com/rs/zerolog/log"
	"net"
	"strings"
	"time"
)

var mqttClient *mqtt.ClientConn

func mqttReader() {
	for {
		conn, err := net.Dial("tcp", "127.0.0.1:1883")
		if err != nil {
			log.Error().Err(err).Send()
		} else {
			mqttClient = mqtt.NewClientConn(conn)
			mqttClient.ClientId = "gw3"
			if err = mqttClient.Connect("", ""); err != nil {
				log.Error().Err(err).Send()
			} else {
				gw.updateInfo()
				mqttClient.Subscribe([]proto.TopicQos{
					{Topic: "gw3/+/set"},
				})
				for m := range mqttClient.Incoming {
					buf := bytes.Buffer{}
					if err = m.Payload.WritePayload(&buf); err != nil {
						log.Error().Err(err).Send()
						continue
					}

					items := strings.Split(m.TopicName, "/")
					if len(items) == 3 && items[2] == "set" {
						mac := items[1]
						if device, ok := devices[mac]; ok {
							device.(DeviceGetSet).setState(buf.Bytes())
						}
					}
				}
			}
			mqttClient = nil
		}
		time.Sleep(time.Second)
	}
}

func mqttPublish(topic string, data interface{}, retain bool) {
	if mqttClient == nil {
		return
	}

	var payload []byte

	switch data.(type) {
	case []byte:
		payload = data.([]byte)
	case string:
		payload = []byte(data.(string))
	default:
		var err error
		if payload, err = json.Marshal(data); err != nil {
			log.Warn().Err(err).Send()
			return
		}
	}

	//var re = regexp.MustCompile(`([0-9A-F]{2}:[0-9A-F]{2}:[0-9A-F]{2}):[0-9A-F]{2}:[0-9A-F]{2}:[0-9A-F]{2}`)
	//topic = re.ReplaceAllString(topic, `$1:FF:FF:FF`)
	//payload = re.ReplaceAll(payload, []byte(`$1:FF:FF:FF`))

	msg := &proto.Publish{
		Header:    proto.Header{Retain: retain},
		TopicName: topic,
		Payload:   proto.BytesPayload(payload),
	}
	mqttClient.Publish(msg)
}

type mqttLogWriter struct{}

func (m mqttLogWriter) Write(p []byte) (n int, err error) {
	if mqttClient != nil {
		msg := &proto.Publish{
			Header:    proto.Header{},
			TopicName: "gw3/log",
			Payload:   proto.BytesPayload(p),
		}
		mqttClient.Publish(msg)
	}

	return len(p), nil
}
