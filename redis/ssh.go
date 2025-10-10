package redis

import "context"

func (r *Redis) NodeConnectable(ctx context.Context, nodeid string) (bool, error) {
	return r.client.SIsMember(ctx, RedisKeyNodeSSHList, nodeid).Result()
}

func (r *Redis) AddNodeToSSHList(ctx context.Context, nodeid string) error {
	return r.client.SAdd(ctx, RedisKeyNodeSSHList, nodeid).Err()
}

func (r *Redis) RemoveNodeFromSSHList(ctx context.Context, nodeid string) error {
	return r.client.SRem(ctx, RedisKeyNodeSSHList, nodeid).Err()
}

func (r *Redis) GetSSHConn(ctx context.Context, nodeid string) (string, error) {
	return r.client.Get(ctx, RedisKeyNodeSShConfig).Result()
}
