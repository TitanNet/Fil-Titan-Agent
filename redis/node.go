package redis

import (
	"agent/redis/metrics"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Node struct {
	ID                  string `redis:"id"`
	UUID                string `redis:"uuid"`
	AndroidID           string `redis:"androidId"`
	AndroidSerialNumber string `redis:"androidSerialNumber"`

	OS              string `redis:"os"`
	Platform        string `redis:"platform"`
	PlatformVersion string `redis:"platformVersion"`
	Arch            string `redis:"arch"`
	BootTime        int64  `redis:"bootTime"`

	Macs string `redis:"macs"`

	CPUModuleName string  `redis:"cpuModuleName"`
	CPUCores      int     `redis:"cpuCores"`
	CPUMhz        float64 `redis:"cpuMhz"`
	CPUUsage      float64 `redis:"cpuUsage"`

	Gpu string `redis:"gpu"`

	TotalMemory     int64  `redis:"totalMemory"`
	UsedMemory      int64  `redis:"usedMemory"`
	AvailableMemory int64  `redis:"availableMemory"`
	MemoryModel     string `redis:"memoryModel"`

	NetIRate float64 `redis:"netIRate"`
	NetORate float64 `redis:"netORate"`

	Baseboard string `redis:"baseboard"`

	TotalDisk int64  `redis:"totalDisk"`
	FreeDisk  int64  `redis:"freeDisk"`
	DiskModel string `redis:"diskModel"`

	LastActivityTime time.Time `redis:"lastActivityTime"`

	// Controller *Controller

	IP string `redis:"ip"`

	// AppList []*App

	// WorkingDir string
	Version string `redis:"version"`
	Channel string `redis:"channel"`

	ServiceState int `redis:"serviceState"`

	CGroup      int    `redis:"cgroup"` // 0 unchecked, 1 enable, 2 disable
	CGroupOut   string `redis:"cgroupOut"`
	Iptables    int    `redis:"iptables"` // 0 unchecked, 1 installed, 2 not installed
	IptablesOut string `redis:"iptablesOut"`
}

// currently android app is acting like this
func (n *Node) NodeIsAndroidApp() bool {
	if n == nil {
		return false
	}
	return n.OS == "Android" && n.Platform == "Linux"
}

func (n *Node) SetCGroup(m string) {
	ms := metrics.VPSMetricString(m)
	enable, out, err := ms.EnableCgroup()
	if err != nil {
		n.CGroup = 0
		n.CGroupOut = err.Error()
		return
	}

	if enable {
		n.CGroup = 1
	} else {
		n.CGroup = 2
	}
	n.CGroupOut = out
}

func (n *Node) SetIptables(m string) {
	ms := metrics.VPSMetricString(m)
	installed, out, err := ms.InstallIptables()
	if err != nil {
		n.Iptables = 0
		n.IptablesOut = err.Error()
		return
	}

	if installed {
		n.Iptables = 1
	} else {
		n.Iptables = 2
	}
	n.IptablesOut = out
}

func (n *Node) GetCGroup() (int, string) {
	return n.CGroup, n.CGroupOut
}

func (n *Node) GetIptables() (int, string) {
	return n.Iptables, n.IptablesOut
}

var InitStateAfterFetchingClientIDMap = map[string]int{
	"pedge":      BizStatusCodeResourceWaitAudit,
	"niulinkant": BizStatusCodeWaitAudit,
	"painet":     BizStatusCodeResourceWaitAudit,

	"vmbox:":       BizStateReservedRunning,
	"emc-titan-l2": BizStateReservedRunning,
}

func GetStateBeforeInit(locationMatch, resourceMatch bool) int {
	// if !locationMatch {
	// 	return BizStatusCodeAreaUnsupport
	// }
	// if !resourceMatch && locationMatch {
	// 	return BizStatusCodeNoTask
	// }
	if !resourceMatch {
		return BizStatusCodeNoTask
	}

	// locationMatch && areaMatch
	return BizStateIniting
}

const (
	BizStatusCodeWaitAudit         = 0  // 资源审核中 正在审核资源中，预计5-20分钟，审核通过后将会自动部署。 (七牛云)
	BizStatusCodeResourceWaitAudit = 2  // 资源审核中 正在审核资源中，预计12-24小时，审核通过后会自动部署 (派享)
	BizStatusCodeAreaUnsupport     = 5  // 部署失败-区域未开放 资源所在地尚未开放需求，请更换地区参与。
	BizStatusCodeNoTask            = 7  // 部署失败-无任务 当前地区无对应设备的资源需求，请更换地区或设备参与.
	BizStateIniting                = 11 // 环境准备中
	BizStatusCodeErr               = 12 // 错误 (获取bizid失败, multipass失败)

	// AgentServer state reserved
	BizStateReservedRunning = 100

	BizStatusRunning = "running"
)

func (r *Redis) SetNode(ctx context.Context, n *Node) error {
	if n == nil {
		return fmt.Errorf("Redis.SetNode: node can not empty")
	}

	if len(n.ID) == 0 {
		return fmt.Errorf("Redis.SetNode: node ID can not empty")
	}

	if len(n.AndroidSerialNumber) > 0 {
		if err := r.client.Set(ctx, fmt.Sprintf(RedisKeySNNode, n.AndroidSerialNumber), n.ID, 0).Err(); err != nil {
			return err
		}
	}

	key := fmt.Sprintf(RedisKeyNode, n.ID)
	err := r.client.HSet(ctx, key, n).Err()
	if err != nil {
		return err
	}

	err = r.client.ZAdd(ctx, RedisKeyZsNodeLastActiveTime, redis.Z{
		Score:  float64(n.LastActivityTime.Unix()),
		Member: n.ID,
	}).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *Redis) GetNode(ctx context.Context, nodeID string) (*Node, error) {
	if len(nodeID) == 0 {
		return nil, fmt.Errorf("Redis.GetNode: nodeID can not empty")
	}

	key := fmt.Sprintf(RedisKeyNode, nodeID)
	res := r.client.HGetAll(ctx, key)
	if res.Err() != nil {
		return nil, res.Err()
	}

	var n Node
	if err := res.Scan(&n); err != nil {
		return nil, err
	}

	if n.ID == "" {
		return nil, redis.Nil
	}

	return &n, nil
}

func (r *Redis) GetNodes(ctx context.Context, nodeIDs []string) ([]*Node, error) {
	if len(nodeIDs) == 0 {
		return nil, fmt.Errorf("Redis.GetNodes: nodeIDs can not empty")
	}

	pipe := r.client.Pipeline()

	var cmds []*redis.MapStringStringCmd
	for _, nodeid := range nodeIDs {
		key := fmt.Sprintf(RedisKeyNode, nodeid)
		cmd := pipe.HGetAll(ctx, key)
		cmds = append(cmds, cmd)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0)
	for _, cmd := range cmds {
		var node Node
		if err := cmd.Scan(&node); err != nil {
			return nil, err
		}
		nodes = append(nodes, &node)
	}

	return nodes, nil
}

func (r *Redis) GetNodeRegistMap(ctx context.Context, nodesArr []string) (map[string]*NodeRegistInfo, error) {
	if len(nodesArr) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeRegistMap: nodesArr can not empty")
	}

	pipe := r.client.Pipeline()
	cmdMap := make(map[string]*redis.StringCmd, len(nodesArr))

	for _, nodeid := range nodesArr {
		cmd := pipe.HGet(ctx, RedisKeyNodeRegist, nodeid)
		cmdMap[nodeid] = cmd
	}

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	rgm := make(map[string]*NodeRegistInfo)
	for nodeid, cmd := range cmdMap {
		var rg NodeRegistInfo

		jsonData := cmd.Val()
		if jsonData == "" {
			rgm[nodeid] = nil
			continue
		}

		if err := json.Unmarshal([]byte(cmd.Val()), &rg); err != nil {
			return nil, err
		}
		// if err := cmd.Scan(&rg); err != nil {
		// 	return nil, err
		// }
		rgm[nodeid] = &rg
	}

	return rgm, nil
}

func (r *Redis) GetNodesAfter(ctx context.Context, t int64) ([]string, error) {
	arr, err := r.client.ZRangeByScore(ctx, RedisKeyZsNodeLastActiveTime, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", t),
		Max: fmt.Sprintf("%d", 4070908800), // 2099-01-01 00:00:00
	}).Result()

	if err != nil {
		return nil, err
	}

	var ret []string
	for _, key := range arr {
		// RedisKeyZsNodeAppMember  = "%s@%s" // app@nodeid
		if strings.Contains(key, "@") {
			an := strings.Split(key, "@")
			if len(an) > 1 {
				ret = append(ret, an[1])
			}
		} else {
			// former member is nodeid
			ret = append(ret, strings.TrimSpace(key))
		}
	}
	return ret, nil

}

func (r *Redis) GetNodeList(ctx context.Context, lastActiveTime time.Time, nodeid string) ([]*Node, error) {

	var (
		cursor uint64
		ret    []*Node
	)

	nodeLike := fmt.Sprintf("%s*", nodeid)
	nodeKeyPattern := strings.Replace(RedisKeyNode, "%s", nodeLike, -1)
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, nodeKeyPattern, 100).Result()
		if err != nil {
			fmt.Println("Error scanning keys:", err)
			break
		}

		for _, key := range keys {
			res := r.client.HGetAll(ctx, key)
			if res.Err() != nil {
				// return nil, res.Err()
				log.Printf("Error HGetAll: %v", res.Err())
				continue
			}

			var n Node
			if err := res.Scan(&n); err != nil {
				// return nil, err
				log.Printf("Error scan node: %v", err)
				continue
			}

			if n.LastActivityTime.After(lastActiveTime) {
				ret = append(ret, &n)
			}

		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return ret, nil
}

func (r *Redis) IncrNodeOnlineDuration(ctx context.Context, nodeid string, seconds int) error {
	if len(nodeid) == 0 {
		return fmt.Errorf("Redis.IncrNodeOnlineDuration: nodeID can not empty")
	}
	if seconds <= 0 {
		return fmt.Errorf("Redis.IncrNodeOnlineDuration: seconds can not less than or equal to zero")
	}
	totalOnlineKey := fmt.Sprintf(RedisKeyNodeOnlineDuration, nodeid)
	if err := r.client.IncrBy(ctx, totalOnlineKey, int64(seconds)).Err(); err != nil {
		return err
	}

	statMapKey := fmt.Sprintf(RedisKeyNodeOnlineDurationStatMap, nodeid)
	today := time.Now().Format("20060102")
	return r.client.HIncrBy(ctx, statMapKey, today, int64(seconds)).Err()
}

func (r *Redis) GetNodeOnlineDuration(ctx context.Context, nodeid string) (int64, error) {
	if len(nodeid) == 0 {
		return 0, fmt.Errorf("Redis.GetNodeOnlineDuration: nodeID can not empty")
	}
	key := fmt.Sprintf(RedisKeyNodeOnlineDuration, nodeid)
	return r.client.Get(ctx, key).Int64()
}

func (r *Redis) GetNodeOnlineDurationMap(ctx context.Context, nodes []string) (map[string]int64, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeOnlineDurationMap: nodes can not empty")
	}

	pipe := r.client.Pipeline()

	cmds := make(map[string]*redis.StringCmd, len(nodes))
	for _, nodeid := range nodes {
		key := fmt.Sprintf(RedisKeyNodeOnlineDuration, nodeid)
		cmds[nodeid] = pipe.Get(ctx, key)
	}
	// fmt.Println(cmds)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		// fmt.Println(err.Error())
		return nil, err
	}

	result := make(map[string]int64, len(nodes))
	for nodeid, cmd := range cmds {
		val, err := cmd.Int64()
		if err != nil {
			if err == redis.Nil {
				result[nodeid] = 0 // Default value if key doesn't exist
				continue
			}
			return nil, fmt.Errorf("Redis.GetNodeOnlineDurationMap parse result failed for node %s: %w", nodeid, err)
		}
		result[nodeid] = val
	}

	return result, nil
}

func (r *Redis) GetNodeOnlineDurationStastics(ctx context.Context, nodeid string) ([]map[string]int64, error) {
	if nodeid == "" {
		return nil, fmt.Errorf("Redis.GetNodeOnlineDurationStastics: nodeID can not empty")
	}

	statMapKey := fmt.Sprintf(RedisKeyNodeOnlineDurationStatMap, nodeid)

	values, err := r.client.HGetAll(ctx, statMapKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to HGetAll from redis: %w", err)
	}

	var ret []map[string]int64
	for date, strVal := range values {
		n, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			log.Printf("Error parsing int from value for date %s: %v", date, err)
			continue
		}

		// avoid overflow
		if n > 86400 {
			n = 86400
		}

		ret = append(ret, map[string]int64{
			date: n,
		})
	}

	return ret, nil

	// var (
	// 	cursor uint64
	// 	ret    []map[string]int64
	// )

	// nodeKeyPattern := fmt.Sprintf(RedisKeyNodeOnlineDurationByDate, nodeid, "*")

	// for {
	// 	keys, nextCursor, err := r.client.Scan(ctx, cursor, nodeKeyPattern, 100).Result()
	// 	if err != nil {
	// 		fmt.Println("Error scanning keys:", err)
	// 		break
	// 	}

	// 	for _, key := range keys {
	// 		res := r.client.Get(ctx, key)
	// 		if res.Err() != nil {
	// 			log.Printf("Error get key %s: %v", key, res.Err())
	// 			continue
	// 		}

	// 		keyArr := strings.Split(key, ":")
	// 		date := keyArr[len(keyArr)-1]

	// 		var n int64
	// 		if err := res.Scan(&n); err != nil {
	// 			log.Printf("Error scan n: %v", err)
	// 			continue
	// 		}

	// 		// todo fix online duration overflow
	// 		if n >= 86400 {
	// 			n = 86400
	// 		}

	// 		ret = append(ret, map[string]int64{
	// 			date: n,
	// 		})
	// 	}

	// 	cursor = nextCursor
	// 	if cursor == 0 {
	// 		break
	// 	}
	// }

	// return ret, nil
}

func (r *Redis) GetNodeOnlineDurationByDate(ctx context.Context, nodeid string, date string) (string, error) {
	if len(nodeid) == 0 || len(date) == 0 {
		return "0", fmt.Errorf("Redis.GetNodeOnlineDurationByDate: nodeID and date must not be empty")
	}

	statMapKey := fmt.Sprintf(RedisKeyNodeOnlineDurationStatMap, nodeid)
	val, err := r.client.HGet(ctx, statMapKey, date).Result()
	if err != nil && err != redis.Nil {
		return "0", err
	}
	if err == redis.Nil {
		return "0", nil
	}
	return val, nil

	// nodeKeyPattern := fmt.Sprintf(RedisKeyNodeOnlineDurationByDate, nodeid, date)
	// res, err := r.client.Get(ctx, nodeKeyPattern).Result()
	// if err != nil && err != redis.Nil {
	// 	return "0", err
	// }
	// if err == redis.Nil {
	// 	return "0", nil
	// }
	// return res, nil

}

func (r *Redis) GetAppConfigByKey(ctx context.Context, key string) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("Redis.GetAppConfigByKey: key must not be empty")
	}
	cfg, err := r.client.Get(ctx, fmt.Sprintf(RedisKeyAppConfig, key)).Result()
	if err != nil && err != redis.Nil {
		return "", err
	}

	return cfg, nil
}

func (r *Redis) NodeInBlackList(ctx context.Context, nodeid string) (bool, error) {
	return r.client.SIsMember(ctx, RedisKeyAgentBlackList, nodeid).Result()
}

func (r *Redis) AddNodeToBlackList(ctx context.Context, nodeid string) error {
	return r.client.SAdd(ctx, RedisKeyAgentBlackList, nodeid).Err()
}

func (r *Redis) RemoveNodeFromBlackList(ctx context.Context, nodeid string) error {
	return r.client.SRem(ctx, RedisKeyAgentBlackList, nodeid).Err()
}
