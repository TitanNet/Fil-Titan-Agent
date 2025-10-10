package server

import (
	"net/url"
	"strconv"
	"time"
)

type Device struct {
	UUID                string `json:"uuid"`
	AndroidID           string `json:"androidID"`
	AndroidSerialNumber string `json:"androidSerialNumber"`

	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platformVersion"`
	Arch            string `json:"arch"`
	BootTime        int64

	Macs string `json:"macs"`

	CPUModuleName string  `json:"cpuModuleName"`
	CPUCores      int     `json:"cpuCores"`
	CPUMhz        float64 `json:"cpuMhz"`
	CPUUsage      float64 `json:"cpuUsage"`
	Gpu           string  `json:"gpu"`

	TotalMemory     int64  `json:"totalmemory"`
	UsedMemory      int64  `json:"usedMemory"`
	AvailableMemory int64  `json:"availableMemory"`
	MemoryModel     string `json:"memoryModel"`

	TotalDisk int64  `json:"totalDisk"`
	FreeDisk  int64  `json:"freeDisk"`
	DiskModel string `json:"diskModel"`

	NetIRate float64 `json:"netIRate"`
	NetORate float64 `json:"netORate"`

	Baseboard string `json:"baseboard"`

	LastActivityTime time.Time `json:"lastActivityTime"`

	//TODO: get controller md5
	ControllerMD5 string `json:"controllerMD5"`

	IP string `json:"ip"`

	AppList []*App `json:"appList"`

	WorkingDir string `json:"workingDir"`
	Channel    string `json:"channel"`

	Version string `json:"version"`
}

func NewDeviceFromURLQuery(values url.Values) *Device {
	d := &Device{LastActivityTime: time.Now()}
	d.UUID = values.Get("uuid")
	d.AndroidID = values.Get("androidID")
	d.AndroidSerialNumber = values.Get("androidSerialNumber")

	d.OS = values.Get("os")
	d.Platform = values.Get("platform")
	d.PlatformVersion = values.Get("platformVersion")
	d.Arch = values.Get("arch")
	d.BootTime = stringToInt64(values.Get("bootTime"))

	d.Macs = values.Get("macs")
	d.CPUModuleName = values.Get("cpuModuleName")
	d.CPUCores = stringToInt(values.Get("cpuCores"))
	d.CPUUsage = stringToFloat64(values.Get("cpuUsage"))
	d.CPUMhz = stringToFloat64(values.Get("cpuMhz"))

	d.Gpu = values.Get("gpu")

	d.TotalMemory = stringToInt64(values.Get("totalmemory"))
	d.UsedMemory = stringToInt64(values.Get("usedMemory"))
	d.AvailableMemory = stringToInt64(values.Get("availableMemory"))
	d.MemoryModel = values.Get("memoryModel")

	d.TotalDisk = stringToInt64(values.Get("totalDisk"))
	d.FreeDisk = stringToInt64(values.Get("freeDisk"))
	d.DiskModel = values.Get("diskModel")

	d.NetIRate = stringToFloat64(values.Get("netIRate"))
	d.NetORate = stringToFloat64(values.Get("netORate"))
	d.Baseboard = values.Get("baseboard")

	d.WorkingDir = values.Get("workingDir")
	d.Channel = values.Get("channel")
	d.Version = values.Get("version")

	return d
}

func stringToInt(v string) int {
	i, _ := strconv.Atoi(v)
	return i
}

func stringToInt64(v string) int64 {
	i, _ := strconv.ParseInt(v, 10, 64)
	return i
}

func stringToFloat64(v string) float64 {
	i, _ := strconv.ParseFloat(v, 64)
	return i
}

// func toRedisNode(d *Device) *redis.Node {
// 	if d.AndroidSerialNumber != "" && redis.BoxSNPattern.MatchString(d.AndroidSerialNumber) {
// 		ok, err := dm.redis.CheckExist(context.Background(), []string{c.AndroidSerialNumber})
// 		if err != nil {
// 			log.Errorf("updateController redis.CheckExist error: %v", err)
// 			return
// 		}
// 		if !ok {
// 			log.Errorf("updateController serialNumber not in whitelist: %s", c.AndroidSerialNumber)
// 			return
// 		}
// 	}

// }
