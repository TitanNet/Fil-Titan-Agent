package redis

import (
	"agent/redis/metrics"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const nodeAppExpireTime = 24 * time.Hour

// App descript the info of app, Does not belong to any node
type App struct {
	AppName string `redis:"appName"`
	// relative app dir
	AppDir     string `redis:"appDir"`
	ScriptName string `redis:"scriptName"`
	ScriptMD5  string `redis:"scriptMD5"`
	Version    string `redis:"version"`
	ScriptURL  string `redis:"scriptURL"`
}

// NodeApp Information that is unique to the node
// Metric includes the app's operational status, as well as unique information
type NodeApp struct {
	AppName          string    `redis:"appName"`
	MD5              string    `redis:"md5"`
	Metric           string    `redis:"metric"`
	LastActivityTime time.Time `redis:"lastActivityTime"`
}

func (redis *Redis) SetApp(ctx context.Context, app *App) error {
	if app == nil {
		return fmt.Errorf("Redis.SetApp: app can not empty")
	}

	if len(app.AppName) == 0 {
		return fmt.Errorf("Redis.SetApp: app name can not empty")
	}

	key := fmt.Sprintf(RedisKeyApp, app.AppName)
	err := redis.client.HSet(ctx, key, app).Err()
	if err != nil {
		return err
	}

	return nil
}

func (redis *Redis) SetApps(ctx context.Context, apps []*App) error {
	pipe := redis.client.Pipeline()
	for _, app := range apps {
		key := fmt.Sprintf(RedisKeyApp, app.AppName)
		pipe.HSet(ctx, key, app).Err()
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (redis *Redis) GetApp(ctx context.Context, appName string) (*App, error) {
	if len(appName) == 0 {
		return nil, fmt.Errorf("Redis.GetApp: nodeID can not empty")
	}

	key := fmt.Sprintf(RedisKeyApp, appName)
	res := redis.client.HGetAll(ctx, key)
	if res.Err() != nil {
		return nil, res.Err()
	}

	var app App
	if err := res.Scan(&app); err != nil {
		return nil, err
	}

	return &app, nil
}

func (r *Redis) GetApps(ctx context.Context, appNames []string) ([]*App, error) {
	pipe := r.client.Pipeline()

	var cmds []*redis.MapStringStringCmd
	for _, appName := range appNames {
		key := fmt.Sprintf(RedisKeyApp, appName)
		cmd := pipe.HGetAll(ctx, key)
		cmds = append(cmds, cmd)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	apps := make([]*App, 0, len(cmds))
	for _, cmd := range cmds {
		var app App
		if err := cmd.Scan(&app); err != nil {
			return nil, err
		}
		apps = append(apps, &app)
	}

	return apps, nil
}

func (redis *Redis) SetNodeApp(ctx context.Context, nodeID string, nApp *NodeApp) error {
	if len(nodeID) == 0 {
		return fmt.Errorf("Redis.SetNodeApp: node id can not empty")
	}
	if nApp == nil {
		return fmt.Errorf("Redis.SetNodeApp: node app can not empty")
	}

	if len(nApp.AppName) == 0 {
		return fmt.Errorf("Redis.SetNodeApp: node app name can not empty")
	}

	nApp.LastActivityTime = time.Now()

	key := fmt.Sprintf(RedisKeyNodeApp, nodeID, nApp.AppName)
	err := redis.client.HSet(ctx, key, nApp).Err()
	if err != nil {
		return err
	}

	err = redis.client.Expire(ctx, key, nodeAppExpireTime).Err()
	if err != nil {
		return err
	}

	return nil
}

func (r *Redis) SetNodeApps(ctx context.Context, nodeID string, nodeApps []*NodeApp) error {
	if len(nodeID) == 0 {
		log.Printf("Redis.SetNodeApp: node id can not empty")
		return nil
	}

	pipe := r.client.Pipeline()

	tn := time.Now()
	for _, app := range nodeApps {
		key := fmt.Sprintf(RedisKeyNodeApp, nodeID, app.AppName)
		app.LastActivityTime = tn
		pipe.HSet(ctx, key, app).Err()
		pipe.ZAdd(ctx, RedisKeyZsNodeAppLastActiveTime, redis.Z{
			Score:  float64(tn.Unix()),
			Member: fmt.Sprintf(RedisKeyZsNodeAppMember, app.AppName, nodeID),
		}).Err()
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (redis *Redis) GetNodeApp(ctx context.Context, nodeID, appName string) (*NodeApp, error) {
	if len(nodeID) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeApp: nodeID can not empty")
	}

	if len(appName) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeApp: node app name can not empty")
	}

	key := fmt.Sprintf(RedisKeyNodeApp, nodeID, appName)
	res := redis.client.HGetAll(ctx, key)
	if res.Err() != nil {
		return nil, res.Err()
	}

	var nApp NodeApp
	if err := res.Scan(&nApp); err != nil {
		return nil, err
	}

	return &nApp, nil
}

func (r *Redis) GetNodeApps(ctx context.Context, nodeID string, appNames []string) ([]*NodeApp, error) {
	if len(nodeID) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeApp: nodeID can not empty")
	}

	pipe := r.client.Pipeline()

	var cmds []*redis.MapStringStringCmd
	for _, appName := range appNames {
		key := fmt.Sprintf(RedisKeyNodeApp, nodeID, appName)
		cmd := pipe.HGetAll(ctx, key)
		cmds = append(cmds, cmd)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	apps := make([]*NodeApp, 0, len(cmds))
	for _, cmd := range cmds {
		var app NodeApp
		if err := cmd.Scan(&app); err != nil {
			return nil, err
		}
		apps = append(apps, &app)
	}

	return apps, nil
}

func (r *Redis) GetNodesApps(ctx context.Context, pairs []NodeAppNamePair, f *AppInfoFileter) ([]*NodeAppExtra, error) {
	pipe := r.client.Pipeline()

	var cmds []*redis.MapStringStringCmd
	for _, pair := range pairs {
		key := fmt.Sprintf(RedisKeyNodeApp, pair.NodeID, pair.AppName)
		cmd := pipe.HGetAll(ctx, key)
		cmds = append(cmds, cmd)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	apps := make([]*NodeAppExtra, 0, len(cmds))
	for i, cmd := range cmds {
		var app NodeAppExtra
		if err := cmd.Scan(&app.NodeApp); err != nil {
			return nil, err
		}
		app.NodeID = pairs[i].NodeID
		apps = append(apps, &app)
	}

	var ret []*NodeAppExtra
	for _, app := range apps {
		if f != nil && f.Match(app) {
			ret = append(ret, app)
		}
	}

	return ret, nil
}

type NodeAppNamePair struct {
	NodeID  string
	AppName string
}

func (r *Redis) GetNodesAppsAfter(ctx context.Context, t int64) ([]NodeAppNamePair, error) {
	list, err := r.client.ZRangeByScore(ctx, RedisKeyZsNodeAppLastActiveTime, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", t),
		Max: fmt.Sprintf("%d", 4070908800), // 2099-01-01 00:00:00
	}).Result()

	if err != nil {
		return nil, err
	}

	pairs := make([]NodeAppNamePair, 0, len(list))
	for _, item := range list {
		pair := strings.Split(item, "@")
		if len(pair) < 2 {
			continue
		}
		pairs = append(pairs, NodeAppNamePair{
			NodeID:  pair[1],
			AppName: pair[0],
		})
	}

	return pairs, nil
}

func (redis *Redis) AddNodeAppsToList(ctx context.Context, nodeID string, appNames []string) error {
	if len(nodeID) == 0 {
		// return fmt.Errorf("Redis.AddNodeApps: node id can not empty")
		return nil
	}

	if len(appNames) == 0 {
		// return fmt.Errorf("Redis.AddNodeApps: node apps name can not empty")
		return nil
	}

	key := fmt.Sprintf(RedisKeyNodeAppList, nodeID)
	err := redis.client.SAdd(ctx, key, appNames).Err()
	if err != nil {
		return err
	}

	return nil
}

func (redis *Redis) DeleteNodeApps(ctx context.Context, nodeID string, appNames []string) error {
	if len(nodeID) == 0 {
		log.Println("Redis.DeleteNodeApp: node id can not empty")
		return nil
	}

	if len(appNames) == 0 {
		log.Println("Redis.DeleteNodeApp: node apps name can not empty")
		return nil
	}

	pipe := redis.client.Pipeline()

	key1 := fmt.Sprintf(RedisKeyNodeAppList, nodeID)
	pipe.SRem(ctx, key1, appNames).Err()

	for _, appName := range appNames {
		key2 := fmt.Sprintf(RedisKeyNodeApp, nodeID, appName)
		pipe.Del(context.Background(), key2)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (redis *Redis) GetNodeAppList(ctx context.Context, nodeID string) ([]string, error) {
	if len(nodeID) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeAppList: nodeID can not empty")
	}

	key := fmt.Sprintf(RedisKeyNodeAppList, nodeID)
	appNames, err := redis.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return appNames, nil
}

type NodeAppExtra struct {
	NodeApp
	NodeID string
}

type AppInfoFileter struct {
	NodeID   string
	Tag      string
	ClientID string
	AppName  string
}

func (f *AppInfoFileter) Match(na *NodeAppExtra) bool {
	if na == nil {
		return false
	}

	return (f.NodeID == "" || f.NodeID == na.NodeID) &&
		(f.AppName == "" || f.AppName == na.AppName)
}

func (r *Redis) GetAppinfosByNodeID(ctx context.Context, nodeid string) ([]*NodeAppExtra, error) {
	if nodeid == "" {
		return nil, fmt.Errorf("GetAppinfosByNodeID: nodeid is required")
	}
	key := fmt.Sprintf(RedisKeyNodeAppList, nodeid)

	apps, err := r.client.SMembers(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if err == redis.Nil {
		return nil, nil
	}

	var ret []*NodeAppExtra

	for _, app := range apps {
		appkey := fmt.Sprintf(RedisKeyNodeApp, nodeid, app)
		res := r.client.HGetAll(ctx, appkey)
		if res.Err() != nil {
			log.Printf("Error HGetAll: %v", res.Err())
			continue
		}

		var n NodeAppExtra
		if err := res.Scan(&n.NodeApp); err != nil {
			log.Printf("Error scan node: %v", err)
			continue
		}

		n.NodeID = nodeid

		ret = append(ret, &n)
	}

	return ret, nil
}

func (r *Redis) GetAllAppInfos(ctx context.Context, lastActiveTime time.Time, f AppInfoFileter) ([]*NodeAppExtra, error) {

	var (
		cursor uint64
		ret    []*NodeAppExtra
	)

	nodeAppKeyPattern := strings.Replace(RedisKeyNodeApp, "%s:%s", "*", -1)
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, nodeAppKeyPattern, 100).Result()
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

			var (
				n NodeAppExtra
			)

			if err := res.Scan(&n.NodeApp); err != nil {
				// return nil, err
				log.Printf("Error scan node: %v", err)
				continue
			}

			//titan:agent:nodeApp:%s:%s
			n.NodeID = strings.Split(key, ":")[3]

			if f.NodeID != "" && f.NodeID != n.NodeID {
				continue
			}

			if f.AppName != "" && f.AppName != n.AppName {
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

func (r *Redis) GetNodeIDBySN(ctx context.Context, sn string) (string, error) {
	key := fmt.Sprintf(RedisKeySNNode, sn)
	return r.client.Get(ctx, key).Result()
}

// returns int: 0= 1=on 2:off , string: cgroup output
func (r *Redis) CheckVPSCGroupInfo(ctx context.Context, nodeid string, appName string) (int, string) {
	app, err := r.GetNodeApp(ctx, nodeid, appName)
	if err != nil {
		log.Printf("CheckVPSCGroupInfo.GetNodeApp error: %v", err)
		return 0, ""
	}
	if app.AppName == "" {
		return 0, ""
	}
	s := metrics.VPSMetricString(app.Metric)
	ok, cginfo, err := s.EnableCgroup()
	if err != nil {
		log.Printf("CheckVPSCGroupInfo.EnableCgroup error: %v", err)
	}
	rt := 2
	if ok {
		rt = 1
	}
	return rt, cginfo
}

func (r *Redis) CheckIptablesInfo(ctx context.Context, nodeid string, appName string) (int, string) {
	app, err := r.GetNodeApp(ctx, nodeid, appName)
	if err != nil {
		log.Printf("CheckIptablesInfo.GetNodeApp error: %v", err)
		return 0, ""
	}
	if app.AppName == "" {
		return 0, ""
	}
	s := metrics.VPSMetricString(app.Metric)
	ok, iptables, err := s.InstallIptables()
	if err != nil {
		log.Printf("CheckIptablesInfo.InstallIptables error: %v", err)
	}
	rt := 2
	if ok {
		rt = 1
	}
	return rt, iptables
}
