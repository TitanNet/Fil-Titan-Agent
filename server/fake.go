package server

import (
	"agent/redis"
	"math"
)

func calUsage(node *redis.Node) {
	if node.CPUCores == 0 && node.CPUUsage > 1 {
		_, node.CPUUsage = math.Modf(node.CPUUsage)
		return
	}
	node.CPUUsage = node.CPUUsage / float64(node.CPUCores)
	if node.CPUUsage > 1 {
		_, node.CPUUsage = math.Modf(node.CPUUsage)
	}
}
