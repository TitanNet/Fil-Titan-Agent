package server

import (
	"agent/common"
	"agent/redis"
	"agent/redis/metrics"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	titanrsa "agent/common/rsa"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
)

func (h *ServerHandler) handleGetAppsConfig(w http.ResponseWriter, r *http.Request) {
	log.Infof("handleGetAppsConfig, queryString %s\n", r.URL.RawQuery)
	payload, err := parseTokenFromRequestContext(r.Context())
	if err != nil {
		log.Infof("ServerHandler.handleGetAppsConfig parseTokenFromRequestContext: %v", err)
		resultError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if ok, _ := h.redis.NodeInBlackList(r.Context(), payload.NodeID); ok {
		log.Infof("ServerHandler.handleGetAppsConfig NodeID %s is in blacklist", payload.NodeID)
		resultError(w, http.StatusUnauthorized, "Node is in blacklist")
		return
	}

	d := NewDeviceFromURLQuery(r.URL.Query())

	uuid := r.URL.Query().Get("uuid")
	channel := r.URL.Query().Get("channel")

	var testApps []string
	testNode := h.config.TestNodes[uuid]
	if testNode != nil {
		testApps = testNode.Apps
	}

	// https://test4-api.titannet.io/config/apps?os=windows
	appList := make([]*AppConfig, 0, len(h.config.AppList))
	for _, app := range h.config.AppList {
		var locationMatch, locationExcludeMatch, resourceMatch bool

		// test case
		if len(testApps) > 0 {
			if h.isTestApp(app.AppName, testApps) {
				appList = append(appList, app)
			}
			continue
		} else if len(channel) > 0 {
			// specify channel case
			// todo channel的arch适配

			resourceMatch = h.isAppMatchChannel(app.AppName, channel)
			if !resourceMatch {
				continue
			}

			locationMatch = h.isMatchLocationApp(r, app.ReqLocations)
			if !locationMatch {
				continue
			}

			locationExcludeMatch = h.isMatchExcludeLocation(r, app.ReqLocationsExclude)
			if locationExcludeMatch {
				continue
			}

			appList = append(appList, app)

			continue
		} else {
			// common case
			if !app.AutoLoad {
				continue
			}
			resourceMatch = h.isResourceMatchApp(r, app.ReqResources)
			if !resourceMatch {
				continue
			}
			locationMatch = h.isMatchLocationApp(r, app.ReqLocations)
			if !locationMatch {
				continue
			}
			locationExcludeMatch = h.isMatchExcludeLocation(r, app.ReqLocationsExclude)
			if locationExcludeMatch {
				continue
			}
			appList = append(appList, app)
			continue
		}
	}

	// |--------|---------|--------|
	// |        | 资源满足 |资源不满足|
	// |--------|---------|--------|
	// | 区域满足|  ✅     | 无任务  |
	// |--------|---------|--------|
	// |区域不满足|区域未开放|区域未开放|
	// |--------|---------|--------|
	var (
		initState = -1
		// bizStatusCodeAreaUnsupport bool
		// bizStatusCodeNoTask        bool
	)

	m := h.getPCDNLocations()
	country, _ := getLocationCountry(getClientIP(r))
	if !m[country] {
		initState = redis.BizStatusCodeAreaUnsupport
	} else {
		for _, app := range h.config.AppList {
			var locationMatch, locationExcludeMatch, resourceMatch bool

			// test case
			if len(testApps) > 0 {
				if h.isTestApp(app.AppName, testApps) {
					initState = redis.BizStateIniting
					break
				}
			} else if len(channel) > 0 {
				// specify channel case
				locationMatch = h.isMatchLocationApp(r, app.ReqLocations)
				resourceMatch = h.isAppMatchChannel(app.AppName, channel)
				locationExcludeMatch = h.isMatchExcludeLocation(r, app.ReqLocationsExclude)
				initState = redis.GetStateBeforeInit(locationMatch && !locationExcludeMatch, resourceMatch)
				if locationMatch && !locationExcludeMatch && resourceMatch {
					break
				}
			} else {
				// common case
				locationMatch = h.isMatchLocationApp(r, app.ReqLocations)
				resourceMatch = h.isResourceMatchApp(r, app.ReqResources)
				locationExcludeMatch = h.isMatchExcludeLocation(r, app.ReqLocationsExclude)
				initState = redis.GetStateBeforeInit(locationMatch && !locationExcludeMatch, resourceMatch)
				if locationMatch && !locationExcludeMatch && resourceMatch && app.AutoLoad {
					break
				}
			}
		}
	}

	// 命中: 则按照命中的状态
	// 未命中:
	// 区域未开放(1) 无任务(0) -> 区域未开放
	// 区域未开放(0) 无任务(1) -> 无任务
	// 区域未开放(1) 无任务(1) -> 区域未开放
	// 区域未开放(0) 无任务(0) -> 保留状态
	// if initState != redis.BizStateIniting {
	// 	if bizStatusCodeAreaUnsupport {
	// 		initState = redis.BizStatusCodeAreaUnsupport
	// 	} else if bizStatusCodeNoTask && !bizStatusCodeAreaUnsupport {
	// 		initState = redis.BizStatusCodeNoTask
	// 	} else {
	// 		initState = redis.BizStateReservedRunning
	// 	}
	// }

	// mergeTestApps(payload.NodeID, &appList)
	// replaceIfTestApp(payload.NodeID, &appList)

	// handle specified apps to replace default setup
	specifiedApps, err := h.handleSpecifiedApps(r.Context(), payload.NodeID)
	if err != nil {
		log.Errorf("handleGetAppsConfig.handleSpecifiedApps failed, nodeid: %s, err: %s", payload.NodeID, err.Error())
	}
	if len(specifiedApps) > 0 {
		appList = specifiedApps
	}

	// handle specified extra apps to add into default setup
	extraApps, err := h.handleSpecifiedExtraApps(r.Context(), payload.NodeID)
	if err != nil {
		log.Errorf("handleGetAppsConfig.handleSpecifiedExtraApps failed, nodeid: %s, err: %s", payload.NodeID, err.Error())
	}
	if len(extraApps) > 0 {
		appList = append(appList, extraApps...)
	}

	// handle removed apps to remove from default setup
	removedApps, err := h.handleRemovedApps(r.Context(), payload.NodeID)
	if err != nil {
		log.Errorf("handleGetAppsConfig.handleRemovedApps failed, nodeid: %s, err: %s", payload.NodeID, err.Error())
	}

	removedAppNames := make(map[string]bool)
	for _, app := range removedApps {
		removedAppNames[app.AppName] = true
	}

	if len(removedAppNames) > 0 {
		filteredAppList := make([]*AppConfig, 0, len(appList))
		for _, app := range appList {
			if !removedAppNames[app.AppName] {
				filteredAppList = append(filteredAppList, app)
			}
		}
		appList = filteredAppList
	}

	// usually (matched + extra - removed) or (specified + extra - removed)
	appListStr, _ := json.Marshal(appList)
	log.Infof("GetAppList node: %s, os: %s, channel: %s, apps: %s", payload.NodeID, r.URL.Query().Get("os"), channel, appListStr)

	node, _ := h.redis.GetNode(context.Background(), payload.NodeID)
	var serviceState int

	// 已经跑起来 到了审核阶段 并且获取到的是初始化阶段 就不应该发生变化
	if node != nil && (node.ServiceState == redis.BizStatusCodeWaitAudit || node.ServiceState == redis.BizStatusCodeResourceWaitAudit) && initState == redis.BizStateIniting {
		serviceState = node.ServiceState
	} else {
		serviceState = initState
	}

	w.Header().Set("ServiceState", strconv.Itoa(serviceState))
	w.Header().Set("InitState", strconv.Itoa(initState))

	recordRawIP(payload.NodeID, r)
	ip := getClientIP(r)
	if ip != "" && node != nil {
		node.IP = ip
		d.IP = ip
	}

	// h.redis.SetNode()
	// h.devMgr.device
	h.devMgr.updateNodeFromDevice(r.Context(), payload.NodeID, d, serviceState)
	// h.devMgr.updateController(&Controller{Device: *d, NodeID: payload.NodeID}, serviceState)

	var appNames []string
	for _, app := range appList {
		appNames = append(appNames, app.AppName)
	}

	if r.URL.Query().Get("os") == "Android" {
		appNames = []string{"qiniu-android"}
	}

	if err := h.redis.AddNodeAppsToList(r.Context(), payload.NodeID, appNames); err != nil {
		log.Errorf("ServerHandler.handleGetAppsConfig AddNodeAppsToList: %v", err)
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	// if err := h.redis.SetNodeSpecifiedApps(r.Context(), payload.NodeID, appNames); err != nil {
	// 	log.Errorf("ServerHandler.handleGetAppsConfig SetNodeSpecifiedApps: %v", err)
	// 	resultError(w, http.StatusInternalServerError, err.Error())
	// 	return
	// }

	buf, err := json.Marshal(appList)
	if err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Write(buf)
}

var (
	locationOnce    sync.Once
	pcdnLocationMap = make(map[string]bool)
)

func (h *ServerHandler) getPCDNLocations() map[string]bool {
	locationOnce.Do(func() {
		for _, app := range h.config.AppList {
			if strings.Contains(app.AppName, "airship") || strings.Contains(app.AppName, "pedge") || strings.Contains(app.AppName, "qiniu") {
				for _, location := range app.ReqLocations {
					pcdnLocationMap[location] = true
				}
			}
		}
	})
	return pcdnLocationMap
}

func (h *ServerHandler) handleSpecifiedApps(ctx context.Context, nodeid string) ([]*AppConfig, error) {
	appNames, err := h.redis.GetNodeSpecifiedApps(ctx, nodeid)
	if err != nil {
		return nil, err
	}
	if len(appNames) == 0 {
		return nil, nil
	}

	var appsList []*AppConfig

	for _, appName := range appNames {
		for _, app := range h.config.AppList {
			if app.AppName == appName {
				appsList = append(appsList, app)
			}
		}
	}

	return appsList, nil
}

// extraApps is designed for
func (h *ServerHandler) handleSpecifiedExtraApps(ctx context.Context, nodeid string) ([]*AppConfig, error) {
	appNames, err := h.redis.GetNodeSpecifiedExtraApps(ctx, nodeid)
	if err != nil {
		return nil, err
	}
	if len(appNames) == 0 {
		return nil, nil
	}

	var appsList []*AppConfig

	for _, appName := range appNames {
		for _, app := range h.config.AppList {
			if app.AppName == appName {
				appsList = append(appsList, app)
			}
		}
	}

	return appsList, nil
}

func (h *ServerHandler) handleRemovedApps(ctx context.Context, nodeid string) ([]*AppConfig, error) {
	appNames, err := h.redis.GetNodeRemovedApps(ctx, nodeid)
	if err != nil {
		return nil, err
	}
	if len(appNames) == 0 {
		return nil, nil
	}

	var appsList []*AppConfig

	for _, appName := range appNames {
		for _, app := range h.config.AppList {
			if app.AppName == appName {
				appsList = append(appsList, app)
			}
		}
	}

	return appsList, nil
}

func (h *ServerHandler) isResourceMatchApp(r *http.Request, reqResources []string) bool {
	os, cpu, memoryMB, diskGB, arch := getResource(r)
	for _, reqResourceName := range reqResources {
		reqRes := h.config.Resources[reqResourceName]
		if reqRes == nil {
			continue
		}

		// arch未定义是通用, 或者包含
		if reqRes.Arch != "" && !strings.Contains(reqRes.Arch, arch) {
			continue
		}

		if reqRes.OS == os && cpu >= reqRes.MinCPU && memoryMB >= reqRes.MinMemoryMB && diskGB >= reqRes.MinDiskGB {
			return true
		}
	}
	return false
}

func (h *ServerHandler) isIPMatchLocationApp(ip string, reqLocations []string) bool {
	if reqLocations == nil {
		return true
	}

	country, err := getLocationCountry(ip)
	if err != nil {
		log.Errorf("isMatchLocationApp: %v", err)
		return false
	}

	for _, reqLoc := range reqLocations {
		if reqLoc == country {
			return true
		}
	}

	return false
}

// isMatchExcludeLocation 是否匹配到了排除国, 匹配到了true不下发, 未匹配上false则下发
func (h *ServerHandler) isMatchExcludeLocation(r *http.Request, reqLocationsExclude []string) bool {
	if len(reqLocationsExclude) == 0 {
		return false
	}

	clientIP := getClientIP(r)
	if clientIP == "" {
		return true
	}

	country, err := getLocationCountry(clientIP)
	if err != nil {
		log.Errorf("isMatchLocationApp: %v", err)
		return true
	}

	for _, reqLoc := range reqLocationsExclude {
		if reqLoc == country {
			return true
		}
	}

	return false
}

// isMatchLocationApp 是否匹配到了要求国家
func (h *ServerHandler) isMatchLocationApp(r *http.Request, reqLocations []string) bool {
	if len(reqLocations) == 0 {
		return true
	}

	clientIP := getClientIP(r)
	if clientIP == "" {
		return false
	}
	country, err := getLocationCountry(clientIP)
	if err != nil {
		log.Errorf("isMatchLocationApp: %v", err)
		return false
	}

	for _, reqLoc := range reqLocations {
		if reqLoc == country {
			return true
		}
	}

	return false
}

var ipMapRaw = sync.Map{}

func recordRawIP(nodeid string, r *http.Request) {
	if nodeid == "" {
		return
	}
	var ipRecords = make(map[string][]string)
	ipRecords["X-Original-Forwarded-For"] = append(ipRecords["X-Original-Forwarded-For"], r.Header.Get("X-Original-Forwarded-For"))
	ipRecords["X-Real-IP"] = append(ipRecords["X-Real-IP"], r.Header.Get("X-Real-IP"))
	ipRecords["RemoteAddr"] = append(ipRecords["RemoteAddr"], r.RemoteAddr)
	ipRecords["X-Forwarded-For"] = append(ipRecords["X-Forwarded-For"], r.Header.Get("X-Forwarded-For"))
	ipRecords["X-Remote-Addr"] = append(ipRecords["X-Remote-Addr"], r.Header.Get("X-Remote-Addr"))
	ipMapRaw.Store(nodeid, ipRecords)
}

// var TestWssshApp = AppConfig{
// 	AppName:    "wsssh-unix",
// 	AppDir:     "wsssh",
// 	ScriptName: "wsssh-unix.lua",
// 	ScriptURL:  "https://pcdn.titannet.io/test4/script/wsssh-unix.lua",
// 	ScriptMD5:  "16f9a27fd021ee6c5972168b4f1059f8",
// }

// var TestNodeList = []string{
// 	"73b09cbf-fda9-4a59-93e8-db76ad273d6c",
// }

// func mergeTestApps(nodeid string, apps *[]*AppConfig) {
// 	for _, v := range TestNodeList {
// 		if v == nodeid {
// 			*apps = append(*apps, &TestWssshApp)
// 			break
// 		}
// 	}
// }

// func replaceIfTestApp(nodeid string, apps *[]*AppConfig) {
// 	for _, v := range TestNodeList {
// 		if v == nodeid {
// 			apps = &[]*AppConfig{&TestWssshApp}
// 			break
// 		}
// 	}
// }

var (
	ipCache    sync.Map
	ipCacheTTL = 30 * time.Minute
)

type ipCacheEntity struct {
	location ipLocation
	expireAt time.Time
}

type ipLocation struct {
	Data struct {
		Country string `json:"country"`
	} `json:"data"`
}

func getLocationCountry(ip string) (string, error) {

	now := time.Now()

	if v, ok := ipCache.Load(ip); ok {
		entry := v.(ipCacheEntity)
		if now.Before(entry.expireAt) {
			return entry.location.Data.Country, nil
		}
	}

	resp, err := http.Get(fmt.Sprintf("https://api-test1.container1.titannet.io/api/v2/location?ip=%s", ip))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var location ipLocation
	err = json.Unmarshal(bodyBytes, &location)
	if err != nil {
		return "", err
	}
	country := location.Data.Country

	ipCache.Store(ip, ipCacheEntity{
		location: location,
		expireAt: now.Add(ipCacheTTL),
	})

	return country, nil
}

func (h *ServerHandler) isTestApp(appName string, testAppNames []string) bool {
	if len(testAppNames) == 0 {
		return false
	}

	for _, testAppName := range testAppNames {
		if appName == testAppName {
			return true
		}
	}

	return false
}

func (h *ServerHandler) isAppMatchChannel(appName string, channel string) bool {
	apps := h.config.ChannelApps[channel]
	if len(apps) == 0 {
		return false
	}

	// log.Info("isAppMatchChannel apps", apps, "current app", appName)
	for _, app := range apps {
		if appName == app {
			return true
		}
	}

	return false
}

func (h *ServerHandler) handleControllerList(w http.ResponseWriter, r *http.Request) {
	log.Infof("handleControllerList, queryString %s\n", r.URL.RawQuery)

	controllers := h.devMgr.getControllers()

	result := struct {
		Total       int           `json:"total"`
		Controllers []*Controller `json:"controllers"`
	}{
		Total:       len(controllers),
		Controllers: controllers,
	}

	formattedJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		http.Error(w, "Failed to format JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(formattedJSON)
}

func (h *ServerHandler) handlePushAppInfo(w http.ResponseWriter, r *http.Request) {

	// payload, err := parseTokenFromRequestContext(r.Context())
	// if err != nil {
	// 	resultError(w, http.StatusUnauthorized, err.Error())
	// 	return
	// }

	var (
		uuid      = r.URL.Query().Get("uuid")
		appName   = r.URL.Query().Get("appName")
		client_id = r.URL.Query().Get("client_id")
	)

	if client_id == "" {
		resultError(w, http.StatusBadRequest, "business_id or client_id cannot be empty")
		return
	}

	_, err := h.redis.GetApp(r.Context(), appName)
	if err != nil {
		resultError(w, http.StatusBadRequest, fmt.Sprintf("failed to find app %s, cause: %s", appName, err.Error()))
		return
	}

	// h.redis.GetNodeApps(r.Context(), payload.NodeID)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("CustomHandler.handleAppInfo read body failed: ", err.Error())
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(b) == 0 {
		log.Error("CustomHandler.handleAppInfo read body is empty")
		resultError(w, http.StatusBadRequest, "body is empty")
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(b))

	// Scan and print each line
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}
	log.Infof("uuid:%s, appName:%s\n", uuid, appName)

	// Check for any errors
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading bytes:", err)
	}

	// TODO: add exterInfo to app

}

func (h *ServerHandler) HandleNodeLogin(w http.ResponseWriter, r *http.Request) {
	var (
		nodeid = r.URL.Query().Get("node_id")
		sign   = r.URL.Query().Get("sign")
	)

	if len(nodeid) == 0 {
		resultError(w, http.StatusBadRequest, "no id in query string")
		return
	}

	if len(sign) == 0 {
		resultError(w, http.StatusBadRequest, "no sign in query string")
		return
	}

	signBytes, err := hex.DecodeString(sign)
	if err != nil {
		resultError(w, http.StatusBadRequest, fmt.Sprintf("hex decode sign failed: %s", err.Error()))
		return
	}

	nodeRegistInfo, err := h.redis.GetNodeRegistInfo(r.Context(), nodeid)
	if err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err = verifySignature(nodeRegistInfo, []byte(nodeid), signBytes); err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	payload := common.JwtPayload{
		NodeID: nodeid,
	}

	w.Header().Set("Web-Server", h.config.WebServer)

	tk, err := h.auth.sign(payload)
	if err != nil {
		resultError(w, http.StatusBadRequest, fmt.Sprintf("sign jwt token failed: %s", err.Error()))
		return
	}

	w.Write([]byte(tk))
}

type NodeKeepaliveDeviceInfo struct {
	ID                  string `redis:"id"`
	UUID                string `json:"uuid"`
	AndroidID           string `json:"androidID"`
	AndroidSerialNumber string `json:"androidSerialNumber"`

	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platformVersion"`
	Arch            string `json:"arch"`
	BootTime        int64  `json:"bootTime"`

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
}

type NodeKeepaliveReq struct {
	Node *NodeKeepaliveDeviceInfo `json:"node"`
	Apps []*App                   `json:"apps"`
}

// keepalive is used for
func (h *ServerHandler) HandleNodeKeepalive(w http.ResponseWriter, r *http.Request) {
	payload, err := parseTokenFromRequestContext(r.Context())
	if err != nil {
		resultError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req NodeKeepaliveReq

	node, err := h.redis.GetNode(r.Context(), payload.NodeID)
	if err != nil {
		log.Error("ServerHandler.HandleNodeKeepalive get node failed: ", err.Error())
	}

	var lastActivityTime time.Time
	if node != nil {
		lastActivityTime = node.LastActivityTime
	} else {
		node = &redis.Node{
			ID: payload.NodeID,
		}
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("ServerHandler.HandleNodeKeepalive read body failed: ", err.Error())
		w.Header().Add("next-keepalive-interval", fmt.Sprintf("%d", int(externalBackoffInterval.Seconds())))
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	err = json.Unmarshal(b, &req)
	if err != nil {
		log.Error("ServerHandler.HandleNodeKeepalive unmarshal body failed: ", err.Error())
		w.Header().Add("next-keepalive-interval", fmt.Sprintf("%d", int(externalBackoffInterval.Seconds())))
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	if payload.NodeID != req.Node.ID {
		log.Errorf("node id %s is not match with payload node id %s", req.Node.ID, payload.NodeID)
		w.Header().Add("next-keepalive-interval", fmt.Sprintf("%d", int(externalKeepaliveInterval.Seconds())))
		resultError(w, http.StatusBadRequest, "node id mismatch")
		return
	}

	// var node redis.Node
	if err := copier.Copy(node, req.Node); err != nil {
		log.Error("ServerHandler.HandleNodeKeepalive copy device failed: ", err.Error())
		w.Header().Add("next-keepalive-interval", fmt.Sprintf("%d", int(externalBackoffInterval.Seconds())))
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	ip := getClientIP(r)
	if ip != "" {
		node.IP = ip
	}

	// SetNode
	duration := int(time.Since(lastActivityTime).Seconds())
	if duration > 0 && !lastActivityTime.IsZero() && duration <= int(maxKeepOnlineInterval.Seconds()) {
		if err := h.redis.IncrNodeOnlineDuration(r.Context(), node.ID, duration); err != nil {
			log.Error("ServerHandler.HandleNodeKeepalive incr node online duration failed: ", err.Error())
			w.Header().Add("next-keepalive-interval", fmt.Sprintf("%d", int(externalBackoffInterval.Seconds())))
			resultError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	node.LastActivityTime = time.Now() // make sure external node will not be offline
	if err := h.redis.SetNode(r.Context(), node); err != nil {
		log.Error("ServerHandler.HandleNodeKeepalive set node failed: ", err.Error())
		w.Header().Add("next-keepalive-interval", fmt.Sprintf("%d", int(externalBackoffInterval.Seconds())))
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	// SetNodeApps
	if err := h.updateNodeApps(payload.NodeID, req.Apps); err != nil {
		log.Error("ServerHandler.handlePushMetrics update nodes app failed:", err.Error())
	}

	if node.NodeIsAndroidApp() {
		if err := h.registAndroidApp(r.Context(), payload.NodeID, req.Apps); err != nil {
			log.Error("ServerHandler.handlePushMetrics regist android app failed:", err.Error())
		}
	}

	w.Header().Add("next-keepalive-interval", fmt.Sprintf("%d", int(externalKeepaliveInterval.Seconds())))

}

func (h *ServerHandler) HandleNodeGetConfig(w http.ResponseWriter, r *http.Request) {
	payload, err := parseTokenFromRequestContext(r.Context())
	if err != nil {
		resultError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if payload.NodeID == "" {
		resultError(w, http.StatusBadRequest, "invalid node id")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		resultError(w, http.StatusBadRequest, "invalid key")
		return
	}

	cfg, err := h.redis.GetAppConfigByKey(r.Context(), key)
	if err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Write([]byte(cfg))
}

func (h *ServerHandler) handlePushMetrics(w http.ResponseWriter, r *http.Request) {
	payload, _ := parseTokenFromRequestContext(r.Context())
	// uuid := r.URL.Query().Get("uuid")

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("ServerHandler.handlePushMetrics read body failed: ", err.Error())
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(b) == 0 {
		log.Error("ServerHandler.handlePushMetrics read body is empty")
		resultError(w, http.StatusBadRequest, "body is empty")
		return
	}

	apps := make([]*App, 0)
	err = json.Unmarshal(b, &apps)
	if err != nil {
		log.Error("ServerHandler.handlePushMetrics Unmarshal failed:", err.Error())
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Infof("[PushMetrics] NodeID:%s, apps: %v, body: %s", payload.NodeID, apps, string(b))

	if err := h.updateNodeApps(payload.NodeID, apps); err != nil {
		log.Error("ServerHandler.handlePushMetrics update nodes app failed:", err.Error())
	}

	node, err := h.redis.GetNode(r.Context(), payload.NodeID)
	if err != nil {
		log.Error("ServerHandler.handlePushMetrics get node failed:", err.Error())
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	if node.NodeIsAndroidApp() {
		if err := h.registAndroidApp(r.Context(), payload.NodeID, apps); err != nil {
			log.Error("ServerHandler.handlePushMetrics regist android app failed:", err.Error())
		}
	}

	// c := h.devMgr.getController(payload.NodeID)
	// if c == nil {
	// 	log.Errorf("ServerHandler.handlePushMetrics controller %s not exist", payload.NodeID)
	// 	resultError(w, http.StatusBadRequest, fmt.Sprintf("controller %s not exist", payload.NodeID))
	// 	return
	// }

	recordRawIP(payload.NodeID, r)
	ip := getClientIP(r)
	if ip != "" {
		node.IP = ip
	}

	// set vps(airship)
	var airshipSet bool

	for _, app := range apps {
		if app.Metric == "" {
			continue
		}

		if !airshipSet && (strings.Contains(app.AppName, "airship") || strings.Contains(app.AppName, "pedge")) {
			node.SetCGroup(app.Metric)
			node.SetIptables(app.Metric)
			airshipSet = true
		}

		m := metrics.NewMetricsString(app.Metric, app.Tag)
		status, errStr := m.GetStatus()
		clientid := m.GetClientID()
		if clientid != "" {
			// h.devMgr.updateController(c, redis.InitStateAfterFetchingClientIDMap[app.Tag])
			stateAfterInit := redis.InitStateAfterFetchingClientIDMap[app.Tag]
			if stateAfterInit == 0 && app.Tag != "niulinkant" {
				stateAfterInit = redis.BizStateIniting
			}
			h.devMgr.updateNode(r.Context(), payload.NodeID, node, stateAfterInit)

			// only one app can be running
			break
		}

		if status == "running" {
			// h.devMgr.updateController(c, redis.BizStateIniting)
			h.devMgr.updateNode(r.Context(), payload.NodeID, node, redis.BizStateIniting)
		}

		if errStr != "running" && errStr != "" {
			h.devMgr.updateNode(r.Context(), payload.NodeID, node, redis.BizStatusCodeErr)
			// h.devMgr.updateController(c, redis.BizStatusCodeErr)
		}
	}

}

func (h *ServerHandler) registAndroidApp(ctx context.Context, nodeID string, apps []*App) error {
	var appnames []string
	for _, app := range apps {
		if app.AppName != "" {
			appnames = append(appnames, app.AppName)
		}
	}
	if len(appnames) > 0 {
		if err := h.redis.AddNodeAppsToList(ctx, nodeID, appnames); err != nil {
			log.Errorf("ServerHandler.registAndroidApp AddNodeAppsToList: %v", err)
			return err
		}
	}
	return nil
}

//------- logic changed --------------------
// 1. 拉取旧app的metric
// 2. 如果当前的app没有metric,则保留旧的metric
// 3. 删除所有旧的app
// 4. 保存当前的所有app

// ------- 覆盖全部的指标信息 ------------------
func (h *ServerHandler) updateNodeApps(nodeID string, apps []*App) error {
	nodeApps := make([]*redis.NodeApp, 0, len(apps))
	for _, app := range apps {
		if app.AppName != "" {
			nodeApps = append(nodeApps, &redis.NodeApp{AppName: app.AppName, MD5: app.ScriptMD5, Metric: app.Metric})
		}
	}
	// appNames, err := h.redis.GetNodeAppList(context.Background(), nodeID)
	// if err != nil {
	// 	return err
	// }

	// oldApps, err := h.redis.GetNodeApps(context.Background(), nodeID, appNames)
	// if err != nil {
	// 	return err
	// }

	// oldAppMap := make(map[string]*redis.NodeApp)
	// for _, app := range oldApps {
	// 	oldAppMap[app.AppName] = app
	// }

	// for _, app := range nodeApps {
	// 	if oldApp := oldAppMap[app.AppName]; oldApp != nil {
	// 		if len(app.Metric) != 0 && len(oldApp.Metric) != 0 {
	// 			app.Metric = oldApp.Metric
	// 		}
	// 	}
	// }

	// if err = h.redis.DeleteNodeApps(context.Background(), nodeID, appNames); err != nil {
	// 	return err
	// }

	if err := h.redis.SetNodeApps(context.Background(), nodeID, nodeApps); err != nil {
		return err
	}

	return nil
}

func (h *ServerHandler) HandleNodeRegist(w http.ResponseWriter, r *http.Request) {
	var (
		nodeid = r.URL.Query().Get("node_id")
		pubKey = r.URL.Query().Get("pub_key")
	)

	pubKeyBytes, err := base64.URLEncoding.DecodeString(pubKey)
	if err != nil {
		http.Error(w, "Failed to decode public key from base64", http.StatusBadRequest)
		return
	}

	if len(nodeid) == 0 {
		resultError(w, http.StatusBadRequest, "no id in query string")
		return
	}

	if _, err := titanrsa.Pem2PublicKey(pubKeyBytes); err != nil {
		resultError(w, http.StatusBadRequest, "pub_key is invalid: "+err.Error())
		return
	}

	registedInfo, err := h.redis.GetNodeRegistInfo(r.Context(), nodeid)
	if err == nil {
		if registedInfo.PublicKey != string(pubKeyBytes) {
			if err := h.redis.UpdateNodePublickKey(r.Context(), nodeid, string(pubKeyBytes)); err != nil {
				resultError(w, http.StatusBadRequest, err.Error())
			}
		}
		return
	}

	regInfo := &redis.NodeRegistInfo{
		NodeID:      nodeid,
		PublicKey:   string(pubKeyBytes),
		CreatedTime: time.Now().Unix(),
	}

	if err := h.redis.NodeRegist(r.Context(), regInfo); err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}
}

func (h *ServerHandler) HandleNodeRegistWithWallet(w http.ResponseWriter, r *http.Request) {
	var (
		nodeid = r.URL.Query().Get("node_id")
		pubKey = r.URL.Query().Get("pub_key")
	)

	pubKeyBytes, err := base64.URLEncoding.DecodeString(pubKey)
	if err != nil {
		http.Error(w, "Failed to decode public key from base64", http.StatusBadRequest)
		return
	}

	pubKeyHexString := hex.EncodeToString(pubKeyBytes)

	if len(nodeid) == 0 {
		resultError(w, http.StatusBadRequest, "no id in query string")
		return
	}

	if len(pubKeyBytes) != secp256k1.PubKeySize {
		resultError(w, http.StatusBadRequest, "length of pubkey is incorrect")
		return
	}

	registedInfo, err := h.redis.GetNodeRegistInfo(r.Context(), nodeid)
	if err == nil {
		if registedInfo.Secp256k1PublicKey != pubKeyHexString {
			registedInfo.Secp256k1PublicKey = pubKeyHexString
			if err := h.redis.NodeRegist(r.Context(), registedInfo); err != nil {
				resultError(w, http.StatusBadRequest, err.Error())
			}
		}
		return
	}

	regInfo := &redis.NodeRegistInfo{
		NodeID:             nodeid,
		Secp256k1PublicKey: pubKeyHexString,
		CreatedTime:        time.Now().Unix(),
	}

	if err := h.redis.NodeRegist(r.Context(), regInfo); err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}
}
