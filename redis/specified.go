package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func (r *Redis) GetNodeSpecifiedApps(ctx context.Context, nodeid string) ([]string, error) {
	if len(nodeid) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeSpeifieddApps: nodeID can not empty")
	}

	key := fmt.Sprintf(RedisKeyNodeSpecifiedApps, nodeid)
	apps, err := r.client.SMembers(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("Redis.GetNodeSpeifieddApps: failed to get members: %s", err.Error())
	}

	return apps, err
}

func (redis *Redis) SetNodeSpecifiedApps(ctx context.Context, nodeid string, apps []string) error {
	if len(nodeid) == 0 {
		return fmt.Errorf("Redis.GetNodeSpeifieddApps: nodeID can not empty")
	}

	key := fmt.Sprintf(RedisKeyNodeSpecifiedApps, nodeid)
	if err := redis.client.Del(ctx, key).Err(); err != nil {
		return err
	}

	return redis.client.SAdd(ctx, key, apps).Err()
}

func (r *Redis) GetNodeSpecifiedExtraApps(ctx context.Context, nodeid string) ([]string, error) {
	if len(nodeid) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeSpeifieddApps: nodeID can not empty")
	}

	key := fmt.Sprintf(RedisKeyNodeSpecifiedExtraApps, nodeid)
	apps, err := r.client.SMembers(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("Redis.GetNodeSpeifieddApps: failed to get members: %s", err.Error())
	}

	return apps, err
}

func (r *Redis) GetNodeRemovedApps(ctx context.Context, nodeid string) ([]string, error) {
	if len(nodeid) == 0 {
		return nil, fmt.Errorf("Redis.GetNodeSpeifieddApps: nodeID can not empty")
	}

	key := fmt.Sprintf(RedisKeyNodeRemovedApps, nodeid)
	apps, err := r.client.SMembers(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("Redis.GetNodeSpeifieddApps: failed to get members: %s", err.Error())
	}

	return apps, err
}

func (r *Redis) GetNodeSpecifiedController(ctx context.Context, nodeid string) (string, error) {
	if len(nodeid) == 0 {
		return "", fmt.Errorf("Redis.GetNodeSpeifieddApps: nodeID can not empty")
	}
	key := fmt.Sprintf(RedisKeyNodeSpecifiedController, nodeid)
	controllerStr, err := r.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return "", fmt.Errorf("Redis.GetNodeSpeifieddApps: failed to get members: %s", err.Error())
	}
	return controllerStr, err
}
