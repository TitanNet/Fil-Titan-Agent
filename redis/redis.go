package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(addr, pass string) *Redis {
	if len(addr) == 0 {
		panic("Redis addr can not empty")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass, // no password set
		DB:       0,    // use default DB
	})

	return &Redis{client: client}
}

const (
	RedisKeyApp                  = "titan:agent:app:%s"
	RedisKeyNode                 = "titan:agent:node:%s"
	RedisKeyZsNodeLastActiveTime = "titan:agent:nodeLastActiveTime"

	RedisKeyNodeSpecifiedApps      = "titan:agent:specified:apps:%s"
	RedisKeyNodeSpecifiedExtraApps = "titan:agent:specified:extra:apps:%s"
	RedisKeyNodeRemovedApps        = "titan:agent:removed:apps:%s"

	RedisKeyNodeSpecifiedController = "titan:agent:specified:controller:%s"

	RedisKeyNodeAppList             = "titan:agent:nodeAppList:%s"
	RedisKeyNodeApp                 = "titan:agent:nodeApp:%s:%s"
	RedisKeyZsNodeAppLastActiveTime = "titan:agent:app:nodeLastActiveTime"
	RedisKeyZsNodeAppMember         = "%s@%s" // app@nodeid

	RedisKeyNodeRegist         = "titan:agent:nodeRegist"
	RedisKeyNodeOnlineDuration = "titan:agent:nodeOnlineDuration:%s"

	RedisKeySNNode = "titan:agent:sn:node:%s"

	RedisKeySNWhitList    = "titan:agent:sn:whiteList"
	RedisKeyNodeSSHList   = "titan:agent:node:ssh:list"
	RedisKeyNodeSShConfig = "titan:agent:node:ssh:config"

	RedisKeyNodeOnlineDurationStatMap = "titan:agent:onlineDurationStat:map:%s" // nodeid[day1:duration1, day2:duration2, ...]
	RedisKeyNodeOnlineDurationByDate  = "titan:agent:onlineDurationDate:%s:%s"

	RedisKeyRdsSuperEval   = "titan:agent:redis:super:eval"
	RedisKeyAppConfig      = "titan:app:config:%s"
	RedisKeyAgentBlackList = "titan:agent:blacklist"
)

func (r *Redis) Ping(ctx context.Context) error {
	reidsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if _, err := r.client.Ping(reidsCtx).Result(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) Eval(ctx context.Context, script string, key []string, args ...interface{}) (interface{}, error) {
	return r.client.Eval(ctx, script, key, args...).Result()
}

func (r *Redis) IsSuperEval(ctx context.Context, nodeid string) error {
	n, err := r.client.Get(ctx, RedisKeyRdsSuperEval).Result()
	if err != nil {
		return err
	}

	if n != nodeid {
		return errors.New("not super eval")
	}
	return nil
}
