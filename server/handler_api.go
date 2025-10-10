package server

import (
	"agent/redis"
	"agent/redis/metrics"
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func (h *ServerHandler) handleGetAppList(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("id")

	apps, err := h.redis.GetNodeAppList(context.Background(), uuid)
	if err != nil {
		apiResultErr(w, err.Error())
		return
	}

	result := APIResult{Data: apps}
	buf, err := json.Marshal(result)
	if err != nil {
		log.Error("ServerHandler.handleGetAppList, Marshal: ", err.Error())
		return
	}

	if _, err := w.Write(buf); err != nil {
		log.Error("ServerHandler.handleGetAppList, Write: ", err.Error())
	}

}

func (h *ServerHandler) handleGetAppInfo(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("id")
	appName := r.URL.Query().Get("app")

	// TODO: convert id to uuid format
	// TODO：check app if exist

	app, err := h.redis.GetNodeApp(context.Background(), uuid, appName)
	if err != nil {
		apiResultErr(w, err.Error())
		return
	}

	res := struct {
		AppName string `json:"appName"`
		NodeID  string `json:"nodeID"`
	}{}

	// app.Metric.UnmarshalJSON()

	// err = json.Unmarshal([]byte(app.Metric), &res)
	// if err != nil {
	// 	apiResultErr(w, err.Error())
	// 	return
	// }

	if app.AppName == "titan-l2" && len(res.NodeID) == 0 {
		apiResultErr(w, "titan-l2 not exist")
		return
	}

	res.AppName = app.AppName

	result := APIResult{Data: res}
	buf, err := json.Marshal(result)
	if err != nil {
		log.Error("ServerHandler.handleGetAppList, Marshal: ", err.Error())
		return
	}

	if _, err := w.Write(buf); err != nil {
		log.Error("ServerHandler.handleGetAppList, Write: ", err.Error())
	}
}

type signVerifyRequest struct {
	NodeId  string `json:"nodeId"`
	Sign    string `json:"sign"`
	Content string `json:"content"`
}

func (h *ServerHandler) handleSignVerify(w http.ResponseWriter, r *http.Request) {
	var req signVerifyRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		apiResultErr(w, err.Error())
		return
	}

	if req.NodeId == "" || req.Sign == "" || req.Content == "" {
		apiResultErr(w, "params can not be empty")
		return
	}

	signBytes, err := hex.DecodeString(req.Sign)
	if err != nil {
		apiResultErr(w, fmt.Sprintf("decode sign error %s", err.Error()))
		return
	}

	node, err := h.redis.GetNodeRegistInfo(context.Background(), req.NodeId)
	if err != nil {
		apiResultErr(w, fmt.Sprintf("node %s not exist", req.NodeId))
		return
	}

	err = verifySignature(node, []byte(req.Content), signBytes)
	if err != nil {
		apiResultErr(w, err.Error())
		return
	}

	if err := json.NewEncoder(w).Encode(APIResult{Data: "success"}); err != nil {
		log.Error("ServerHandler.handleSignVerify, Encode: ", err.Error())
	}
}

func (h *ServerHandler) handleGetNodeInfo(w http.ResponseWriter, r *http.Request) {
	nodeid := r.URL.Query().Get("node_id")
	sn := r.URL.Query().Get("sn")
	if nodeid == "" && sn == "" {
		http.Error(w, "node_id or sn parameter required", http.StatusBadRequest)
		return
	}

	var (
		nodeInfo *redis.Node
		appInfo  []*redis.NodeAppExtra
		err      error
	)
	if nodeid != "" {
		nodeInfo, appInfo, err = h.getNodeInfoByNodeID(r.Context(), nodeid)
	}

	if sn != "" {
		nodeInfo, appInfo, err = h.getNodeInfoByAppSN(r.Context(), sn)
	}

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		rsp := APIResult{ErrMsg: err.Error(), Data: nil}
		json.NewEncoder(w).Encode(rsp)
		return
	}

	type NodeInfoRet struct {
		NodeInfo        NodeWebInfo           `json:"node"`
		AppInfo         []*redis.NodeAppExtra `json:"apps"`
		OnlineStatstics []map[string]int64    `json:"onlineStatistics"`
	}

	nodeinfo := NodeWebInfo{
		Node: nodeInfo,
	}
	if time.Since(nodeinfo.LastActivityTime) > offlineTime {
		nodeinfo.State = NodeStateOffline
	} else {
		nodeinfo.State = NodeStateOnline
	}
	nodeinfo.OnlineDuration, _ = h.redis.GetNodeOnlineDuration(r.Context(), nodeinfo.ID)

	nodeinfo.CGroup, nodeinfo.CGroupOut = nodeinfo.GetCGroup()
	nodeinfo.Iptables, nodeinfo.IptablesOut = nodeinfo.GetIptables()

	calUsage(nodeinfo.Node)

	rinfo, _ := h.redis.GetNodeRegistInfo(r.Context(), nodeinfo.ID)
	if rinfo != nil {
		nodeinfo.OnlineRate = float64(nodeinfo.OnlineDuration) / float64(time.Since(time.Unix(rinfo.CreatedTime, 0)).Seconds())
	}
	onlineStatistics, err := h.redis.GetNodeOnlineDurationStastics(r.Context(), nodeinfo.ID)
	if err != nil {
		log.Errorf("GetNodeInfoByNodeID.GetNodeOnlineDurationStastics: %v", err)
	}

	ret := APIResult{Data: NodeInfoRet{NodeInfo: nodeinfo, AppInfo: appInfo, OnlineStatstics: onlineStatistics}}
	json.NewEncoder(w).Encode(ret)
}

func (h *ServerHandler) getNodeInfoByNodeID(ctx context.Context, nodeid string) (*redis.Node, []*redis.NodeAppExtra, error) {
	nodeInfo, err := h.redis.GetNode(ctx, nodeid)
	if err != nil {
		log.Errorf("GetNodeInfoByNodeID.GetNode: %v", err)
		return nil, nil, err
	}

	// appInfo, err := h.redis.GetAllAppInfos(ctx, time.Unix(0, 0), redis.AppInfoFileter{NodeID: nodeid})
	// if err != nil {
	// 	log.Errorf("GetNodeInfoByNodeID.GetAllAppInfos: %v", err)
	// }

	appInfo, err := h.redis.GetAppinfosByNodeID(ctx, nodeid)
	if err != nil {
		log.Errorf("GetNodeInfoByNodeID.GetAllAppInfos: %v", err)
		return nodeInfo, nil, err
	}

	return nodeInfo, appInfo, nil
}

func (h *ServerHandler) getNodeInfoByAppSN(ctx context.Context, sn string) (*redis.Node, []*redis.NodeAppExtra, error) {

	nodeid, err := h.redis.GetNodeIDBySN(ctx, sn)
	if err != nil {
		log.Errorf("GetNodeIDBySN(%s) error(%v)", sn, err)
		return nil, nil, err
	}

	if nodeid == "" {
		return nil, nil, errors.New("nodeid is empty")
	}

	return h.getNodeInfoByNodeID(ctx, nodeid)

}

type NodeWebInfo struct {
	*redis.Node
	State          int   // 0 exception, 1 online, 2 offline
	OnlineDuration int64 // online minutes
	OnlineRate     float64
	CGroup         int // 0 unchecked, 1 enable, 2 disable
	CGroupOut      string
	Iptables       int // 0 unchecked, 1 installed, 2 not installed
	IptablesOut    string
}

const (
	NodeStateException = 0
	NodeStateOnline    = 1
	NodeStateOffline   = 2
)

func (h *ServerHandler) handleGetNodeList(w http.ResponseWriter, r *http.Request) {
	lastActivityTime := r.URL.Query().Get("last_activity_time")
	lastActivityTimeInt, _ := strconv.Atoi(lastActivityTime)

	// latTime, err := time.Parse(time.RFC3339, lastActivityTime)
	// if err != nil {
	// 	apiResultErr(w, "invalid last_activity_time timeformat")
	// 	return
	// }

	// latTime := time.Unix(int64(lastActivityTimeInt), 0)

	nodesArr, err := h.redis.GetNodesAfter(r.Context(), int64(lastActivityTimeInt))
	if err != nil {
		apiResultErr(w, fmt.Sprintf("find node list failed: %s", err.Error()))
		return
	}

	// nodes, err := h.redis.GetNodeList(context.Background(), latTime, nodeid)
	// if err != nil {
	// 	apiResultErr(w, fmt.Sprintf("find node list failed: %s", err.Error()))
	// 	return
	// }

	// var ret = make([]*NodeWebInfo, len(nodesA))
	nodes, err := h.redis.GetNodes(r.Context(), nodesArr)
	if err != nil {
		apiResultErr(w, fmt.Sprintf("ServerHandler.handleGetNodeList, GetNodes: %s", err.Error()))
		return
	}

	durationMap, err := h.redis.GetNodeOnlineDurationMap(r.Context(), nodesArr)
	if err != nil {
		apiResultErr(w, fmt.Sprintf("ServerHandler.handleGetNodeList, GetNodeOnlineDurationMap: %s", err.Error()))
		return
	}

	registMap, err := h.redis.GetNodeRegistMap(r.Context(), nodesArr)
	if err != nil {
		apiResultErr(w, fmt.Sprintf("ServerHandler.handleGetNodeList, GetNodeRegistMap: %s", err.Error()))
		return
	}

	var ret = make([]*NodeWebInfo, len(nodes))

	// var airshipAppNames []string
	// for _, app := range h.config.AppList {
	// 	if strings.Contains(app.AppName, "airship") {
	// 		airshipAppNames = append(airshipAppNames, app.AppName)
	// 	}
	// }

	for i := range ret {
		node := nodes[i]
		n := &NodeWebInfo{Node: node}
		n.CGroup, n.CGroupOut = node.GetCGroup()
		n.Iptables, n.IptablesOut = node.GetIptables()

		if time.Since(node.LastActivityTime) > offlineTime {
			n.State = NodeStateOffline
		} else {
			n.State = NodeStateOnline
		}
		n.OnlineDuration = durationMap[node.ID]
		if rinfo := registMap[node.ID]; rinfo != nil && time.Since(time.Unix(rinfo.CreatedTime, 0)).Seconds() > 0 {
			n.OnlineRate = float64(n.OnlineDuration) / float64(time.Since(time.Unix(rinfo.CreatedTime, 0)).Seconds())
		}

		calUsage(node)
		// handle vps channel
		// if node.Channel == "vps" {

		// 	for _, appName := range airshipAppNames {
		// 		cgroup, cgroupOut := h.redis.CheckVPSCGroupInfo(r.Context(), node.ID, appName)
		// 		if cgroup > 0 && n.CGroup == 0 {
		// 			n.CGroup = cgroup
		// 			n.CGroupOut = cgroupOut
		// 		}

		// 		iptables, iptablesOut := h.redis.CheckIptablesInfo(r.Context(), node.ID, appName)
		// 		if iptables > 0 && n.Iptables == 0 {
		// 			n.Iptables = iptables
		// 			n.IptablesOut = iptablesOut
		// 		}

		// 		if n.CGroup > 0 && n.Iptables > 0 {
		// 			break
		// 		}
		// 	}
		// }
		ret[i] = n
	}

	// for _, nodeid := range nodesArr {
	// 	node, err := h.redis.GetNode(r.Context(), nodeid)
	// 	if err != nil {
	// 		log.Errorf("ServerHandler.handleGetNodeList, GetNode: %s", err.Error())
	// 		continue
	// 	}
	// 	var n = &NodeWebInfo{Node: node}

	// 	if time.Since(node.LastActivityTime) > offlineTime {
	// 		n.State = NodeStateOffline
	// 	} else {
	// 		n.State = NodeStateOnline
	// 	}
	// 	n.OnlineDuration, _ = h.redis.GetNodeOnlineDuration(r.Context(), node.ID)
	// 	rinfo, _ := h.redis.GetNodeRegistInfo(r.Context(), node.ID)
	// 	if rinfo != nil {
	// 		n.OnlineRate = float64(n.OnlineDuration) / float64(time.Since(time.Unix(rinfo.CreatedTime, 0)).Seconds())
	// 	}

	// 	calUsage(node)

	// 	// handle vps channel
	// 	if node.Channel == "vps" {
	// 		n.CGroup, n.CGroupOut = h.redis.CheckVPSCGroupInfo(r.Context(), node.ID, h.config.ChannelApps["vps"][0])
	// 	}

	// 	ret = append(ret, n)
	// }

	result := APIResult{Data: ret}
	buf, err := json.Marshal(result)
	if err != nil {
		log.Error("ServerHandler.handleGetNodeList, Marshal: ", err.Error())
		return
	}

	if _, err := w.Write(buf); err != nil {
		log.Error("ServerHandler.handleGetNodeList, Write: ", err.Error())
	}
}

type NodeAppWebInfo struct {
	// *redis.NodeApp

	LastActivityTime time.Time
	NodeID           string
	AppName          string
	Channel          string
	Tag              string
	ClientID         string
	Status           int
	Err              string
}

func (h *ServerHandler) handleGetAllNodesAppInfosList(w http.ResponseWriter, r *http.Request) {

	lastActivityTime := r.URL.Query().Get("last_activity_time")
	nodeid := r.URL.Query().Get("node_id")
	tag := r.URL.Query().Get("tag")
	clientid := r.URL.Query().Get("client_id")
	appname := r.URL.Query().Get("app_name")

	lastActivityTimeInt, _ := strconv.Atoi(lastActivityTime)

	latTime := time.Unix(int64(lastActivityTimeInt), 0)

	nodeAppsListPairs, err := h.redis.GetNodesAppsAfter(r.Context(), latTime.Unix())
	if err != nil {
		apiResultErr(w, fmt.Sprintf("find nodes apps list pairs failed: %s", err.Error()))
		return
	}

	nodeApps, err := h.redis.GetNodesApps(r.Context(), nodeAppsListPairs, &redis.AppInfoFileter{
		NodeID: nodeid, Tag: tag, ClientID: clientid, AppName: appname,
	})
	if err != nil {
		apiResultErr(w, fmt.Sprintf("find nodes apps list pairs failed: %s", err.Error()))
		return
	}

	// nodeApps, err := h.redis.GetAllAppInfos(r.Context(), latTime, redis.AppInfoFileter{
	// 	NodeID: nodeid, Tag: tag, ClientID: clientid, AppName: appname,
	// })
	// if err != nil {
	// 	apiResultErr(w, fmt.Sprintf("find apps list failed: %s", err.Error()))
	// 	return
	// }

	channelRefMap := make(map[string]string)
	for channel, appNames := range h.config.ChannelApps {
		for _, appName := range appNames {
			channelRefMap[appName] = channel
		}
	}

	tagRefMap := make(map[string]string)
	for _, app := range h.config.AppList {
		tagRefMap[app.AppName] = app.Tag
	}

	var ret []*NodeAppWebInfo = make([]*NodeAppWebInfo, 0)

	for _, nodeApp := range nodeApps {
		tag := tagRefMap[nodeApp.AppName]
		clientid := metrics.GetClientID(nodeApp.Metric, tagRefMap[nodeApp.AppName])
		// 翼兔云 一个app读取多个平台的id
		if isMixedAppClient(clientid) {
			for _, singleClientid := range getMixedAppClient(clientid) {
				newTag := tag
				if newTag == "" {
					newTag = judgeTagByClientID(singleClientid)
				}
				ret = append(ret, &NodeAppWebInfo{
					AppName:          nodeApp.AppName,
					LastActivityTime: nodeApp.LastActivityTime,
					NodeID:           nodeApp.NodeID,
					Channel:          channelRefMap[nodeApp.AppName],
					Tag:              newTag,
					ClientID:         singleClientid,
				})
			}
		} else {
			if tag == "" {
				tag = judgeTagByClientID(clientid)
			}
			ret = append(ret, &NodeAppWebInfo{
				AppName:          nodeApp.AppName,
				LastActivityTime: nodeApp.LastActivityTime,
				NodeID:           nodeApp.NodeID,
				Channel:          channelRefMap[nodeApp.AppName],
				Tag:              tag,
				ClientID:         clientid,
			})
		}
	}

	result := APIResult{Data: ret}
	buf, err := json.Marshal(result)
	if err != nil {
		log.Error("ServerHandler.handleGetAllNodesAppInfosList, Marshal: ", err.Error())
		return
	}

	if _, err := w.Write(buf); err != nil {
		log.Error("ServerHandler.handleGetAllNodesAppInfosList, Write: ", err.Error())
	}
}

func (h *ServerHandler) handleLoadNodeIP(w http.ResponseWriter, r *http.Request) {
	c, _ := ipMapRaw.Load(r.URL.Query().Get("node_id"))
	json.NewEncoder(w).Encode(c)
	// json.NewEncoder(w).Encode()
}

func (h *ServerHandler) handleClearIP(w http.ResponseWriter, r *http.Request) {
	ipMapRaw.Delete(r.URL.Query().Get("ip"))
	json.NewEncoder(w).Encode(APIResult{Data: "success"})
}

func isMixedAppClient(id string) bool {
	return len(strings.Split(id, ";")) > 1
}

func getMixedAppClient(id string) []string {
	return strings.Split(id, ";")
}

func judgeTagByClientID(id string) string {
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "box") {
		return "painet"
	}
	if strings.HasPrefix(id, "ant") {
		return "niulinkant"
	}

	return ""
}

func getClientIP(r *http.Request) string {

	ip := getValidIPFromHeader(r.Header.Get("X-Original-Forwarded-For"))
	if ip != "" {
		return ip
	}

	ip = getValidIPFromHeader(r.Header.Get("X-Forwarded-For"))
	if ip != "" {
		return ip
	}

	ip = getValidIPFromHeader(r.Header.Get("X-Remote-Addr"))
	if ip != "" {
		return ip
	}

	ip = r.Header.Get("X-Real-IP")
	if ip != "" && !isPrivateIP(ip) {
		return ip
	}

	ip, _, _ = net.SplitHostPort(r.RemoteAddr)

	if ip != "" && !isPrivateIP(ip) {
		return ip
	}

	return ""
}

func getValidIPFromHeader(header string) string {
	for _, ip := range strings.Split(header, ",") {
		ip = strings.TrimSpace(ip)
		if ip != "" && !isPrivateIP(ip) {
			return ip
		}
	}
	return ""
}

func isPrivateIP(ipstr string) bool {
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return false
	}

	if ip.To4() != nil {
		ip = ip.To4()

		switch {
		case ip[0] == 10:
			// 10.0.0.0 To 10.255.255.255
			return true
		case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
			// 172.16.0.0 To 172.31.255.255
			return true
		case ip[0] == 192 && ip[1] == 168:
			// 192.168.0.0 To 192.168.255.255
			return true
		}
	}

	return false
}

func (h *ServerHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {

	// 1. check redis
	if err := h.redis.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// 2. xxx

	w.WriteHeader(http.StatusOK)
}

func (h *ServerHandler) handleOnlineDurationByDate(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		resultError(w, http.StatusBadRequest, "date is required")
		return
	}

	list, err := h.redis.GetNodeList(r.Context(), time.Unix(0, 0), "")
	if err != nil {
		apiResultErr(w, err.Error())
		return
	}
	var ret [][]string

	for _, node := range list {
		v, err := h.redis.GetNodeOnlineDurationByDate(r.Context(), node.ID, date)
		if err != nil {
			apiResultErr(w, fmt.Sprintf("failed to get online duration for node %s: %v", node.ID, err))
			return
		}

		ret = append(ret, []string{node.ID, date, v})
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="online_duration.csv"`)
	csvw := csv.NewWriter(w)
	csvw.WriteAll(ret)
	csvw.Flush()
}

func (h *ServerHandler) handleSetNodeConfigs(w http.ResponseWriter, r *http.Request) {

	nodeid := r.URL.Query().Get("node_id")
	if nodeid == "" {
		resultError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	err := h.nodeAppIsSwitchable(r.Context(), nodeid)
	if err != nil {
		resultError(w, http.StatusServiceUnavailable, fmt.Sprintf("switch apps is unavailable, because: %s", err.Error()))
		return
	}

	var setNodeAppReq = make([]*NodeAppConfig, 0)
	if err := json.NewDecoder(r.Body).Decode(&setNodeAppReq); err != nil {
		log.Errorf("failed to decode set node configs request: %v", err)
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	var apps []string
	for _, v := range setNodeAppReq {
		for _, vv := range h.config.AppList {
			if v.AppName == vv.AppName && v.Tag == vv.Tag {
				apps = append(apps, v.AppName)
			}
		}
	}

	if err := h.redis.SetNodeSpecifiedApps(r.Context(), nodeid, apps); err != nil {
		log.Errorf("failed to set node %s configs: %v", nodeid, err)
		resultError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := json.NewEncoder(w).Encode(APIResult{Data: "success"}); err != nil {
		log.Error("ServerHandler.handleSetNodeConfigs, Encode: ", err.Error())
	}

}

// judge node apps is switchable
func (h *ServerHandler) nodeAppIsSwitchable(ctx context.Context, nodeid string) error {
	ackedApps, err := h.redis.GetNodeSpecifiedApps(ctx, nodeid)
	if err != nil {
		log.Errorf("failed to get node apps: %v", err)
		// resultError(w, http.StatusInternalServerError, err.Error())
		return err
	}

	_, appinfos, err := h.getNodeInfoByNodeID(ctx, nodeid)
	if err != nil {
		log.Errorf("failed to get node info: %v", err)
		return err
	}

	var appinfosMap = make(map[string]*redis.NodeAppExtra)
	for _, v := range appinfos {
		appinfosMap[v.AppName] = v
	}

	// make sure acked apps are already exist in appinfos
	for _, v := range ackedApps {
		app, ok := appinfosMap[v]
		if !ok || app == nil {
			return fmt.Errorf("app %s is in process", v)
		}
		// make sure node last switch task is online (avoid of frequency switch)
		if time.Since(app.LastActivityTime) > offlineTime {
			return fmt.Errorf("app %s last task is still offline, please wait till it succeed", v)
		}
	}

	return nil
}

type NodeAppConfig struct {
	NodeID  string `json:"node_id"`
	AppName string `json:"app_name"`
	Tag     string `json:"tag"`
}

func (h *ServerHandler) handleGetNodeConfigs(w http.ResponseWriter, r *http.Request) {
	nodeid := r.URL.Query().Get("node_id")
	if nodeid == "" {
		resultError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	apps, err := h.redis.GetNodeSpecifiedApps(r.Context(), nodeid)
	if err != nil {
		log.Error("handleConfigApp.GetNodeSpecifiedApps error: ", err)
		resultError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// if specified, return
	if len(apps) > 0 {
		appm := make(map[string]*AppConfig)
		for _, a := range h.config.AppList {
			appm[a.AppName] = a
		}

		ret := make([]NodeAppConfig, 0)
		for _, appname := range apps {
			app, ok := appm[appname]
			if !ok {
				continue
			}
			ret = append(ret, NodeAppConfig{
				NodeID:  nodeid,
				AppName: app.AppName,
				Tag:     app.Tag,
			})
		}
		if err := json.NewEncoder(w).Encode(APIResult{Data: ret}); err != nil {
			log.Error("ServerHandler.handleGetNodeConfigs.Specified, Encode: ", err.Error())
		}
		return
	}

	applist, err := h.redis.GetNodeAppList(r.Context(), nodeid)
	if err != nil {
		log.Error("handleConfigApp.GetNodeSpecifiedApps error: ", err)
		resultError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(applist) > 0 {
		appm := make(map[string]*AppConfig)
		for _, a := range h.config.AppList {
			appm[a.AppName] = a
		}

		ret := make([]NodeAppConfig, 0)
		for _, appname := range applist {
			app, ok := appm[appname]
			if !ok {
				continue
			}
			ret = append(ret, NodeAppConfig{
				NodeID:  nodeid,
				AppName: app.AppName,
				Tag:     app.Tag,
			})
		}
		if err := json.NewEncoder(w).Encode(APIResult{Data: ret}); err != nil {
			log.Error("ServerHandler.handleGetNodeConfigs.loaded, Encode: ", err.Error())
		}
		return
	}

	ret := make([]NodeAppConfig, 0)
	json.NewEncoder(w).Encode(APIResult{Data: ret})
}
