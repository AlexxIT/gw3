# gw3

[![Donate](https://img.shields.io/badge/donate-BuyMeCoffee-yellow.svg)](https://www.buymeacoffee.com/AlexxIT)
[![Donate](https://img.shields.io/badge/donate-YooMoney-8C3FFD.svg)](https://yoomoney.ru/to/41001428278477)

Standalone application for integrating **Xiaomi Mijia Smart Multi-Mode Gateway (aka Xiaomi Gateway 3)** into an open source Smart Home platforms.

The application runs directly on the gateway and converts data from surrounding BLE devices into MQTT-topics. All default gateway functionality continues to work as usual in the Mi Home ecosystem.

The app only needs the Internet to download the encryption keys for Xiaomi devices. The rest of the time, the app works without the Internet.

## Supported BLE Devices

BLE devices are not linked to any specific gateway. Their data can be received by multiple gateways at the same time. Remember about the distance between the device and the gateway.

**Mi Home Devices**

All Xiaomi devices must be linked to an account via the Mi Home app, otherwise they don't send data. Some Xioami devices have encryption. The app will automatically retrieve encryption keys for your devices from the cloud. The app will display all unencrypted devices, even if they are connected to another account.

- Aqara Door Lock N100 (ZNMS16LM)
- Aqara Door Lock N200 (ZNMS17LM)
- Honeywell Smoke Alarm (JTYJ-GD-03MI)
- Xiaomi Alarm Clock (CGD1)
- Xiaomi Door Lock (MJZNMS02LM,XMZNMST02YD)
- Xiaomi Door Sensor 2 (MCCGQ02HL)
- Xiaomi Flower Care (HHCCJCY01)
- Xiaomi Flower Pot (HHCCPOT002)
- Xiaomi Kettle (YM-K1501)
- Xiaomi Magic Cube (XMMF01JQD) - doesn't sends edge info, only direction!
- Xiaomi Mosquito Repellent (WX08ZM)
- Xiaomi Motion Sensor 2 (RTCGQ02LM)
- Xiaomi Night Light 2 (MJYD02YL-A)
- Xiaomi Qingping Door Sensor (CGH1)
- Xiaomi Qingping Motion Sensor (CGPR1)
- Xiaomi Qingping TH Lite (CGDK2)
- Xiaomi Qingping TH Sensor (CGG1)
- Xiaomi Safe Box (BGX-5/X1-3001)
- Xiaomi TH Clock (LYWSD02MMC)
- Xiaomi TH Sensor (LYWSDCGQ/01ZM)
- Xiaomi TH Sensor 2 (LYWSD03MMC) - also supports custom [atc1441](https://github.com/atc1441/ATC_MiThermometer) and [pvvx](https://github.com/pvvx/ATC_MiThermometer) firmwares
- Xiaomi Toothbrush T500 (MES601)
- Xiaomi Viomi Kettle (V-SK152)
- Xiaomi Water Leak Sensor (SJWS01LM)
- Xiaomi ZenMeasure Clock (MHO-C303)
- Xiaomi ZenMeasure TH (MHO-C401)
- Yeelight Button S1 (YLAI003)

**Other Xiaomi Devices**

- Xiaomi Mi Scale (XMTZC01HM)
- Xiaomi Mi Scale 2 (XMTZC04HM)
- Yeelight Dimmer (YLKG07YL) - works awful, skips a lot of click
- Yeelight Heater Remote (YLYB01YL-BHFRC)
- Yeelight Remote Control (YLYK01YL)

**Person Trackers**

- [iBeacon](https://en.wikipedia.org/wiki/IBeacon) - example [Home Assistant for Android](https://companion.home-assistant.io/docs/core/sensors#bluetooth-sensors) **BLE Transmitter** feature
- [NutFind Nut](https://www.nutfind.com/) - must be unlinked from the phone
- Amazfit Watch - the detection function must be enabled, it may be enabled not on all accounts
- Xiaomi Mi Band - the detection function must be enabled, enabled by default on some models

## Using with Home Assistant

Just install [Xiaomi Gateway 3](https://github.com/AlexxIT/XiaomiGateway3) integration. It will do all the magic for you.

## Manual installation

If you are not an IT guy, just use Home Assistant. Seriously, why do you need all this trouble?

If you are still here:

1. Open [Telnet](https://gist.github.com/zvldz/1bd6b21539f84339c218f9427e022709) on the gateway.
2. Install [custom firmware](https://github.com/zvldz/mgl03_fw/tree/main/firmware)
   - it will open telnet forever (until the next firmware update)
   - it will run public MQTT on gateway (without auth)
   - it will allow you to run your startup script
3. Download lastest [gw3 binary](https://sourceforge.net/projects/mgl03/files/bin/) on gateway:
   ```shell
   wget "http://master.dl.sourceforge.net/project/mgl03/bin/gw3?viasf=1" -O /data/gw3 && chmod +x /data/gw3
   ```
4. Add your startup script `/data/run.sh` on gateway:
   ```shell
   echo "/data/gw3 -log=syslog,info 2>&1 | mosquitto_pub -t gw3/stderr -s &" > /data/run.sh && chmod +x /data/run.sh
   ```
5. Reboot gateway

**PS:** gw3 binary can run on the original gateway firmware. Custom firmware is an optional step that just makes your life easier. Custom firmware doesn't change default gateway functionality in Mi Home ecosystem.

## Debug

- Support levels: `debug`, `info`, warn (default)
- Support output: `syslog`, `mqtt`, stdout (default)
- Support format: `text` (nocolor), `json`, color (default)
- Support more debug: `btraw`, `btgap`, `miio`

Example: debug logs in json format to MQTT:

```shell
/data/gw3 -log=mqtt,json,debug
```

Or change logs while app running:

```shell
mosquitto_pub -t gw3/AA:BB:CC:DD:EE:FF/set -m '{"log":"syslog,info,text"}'
```
