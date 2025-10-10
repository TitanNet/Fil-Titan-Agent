package server

import (
	"agent/redis"
	"context"
	"net/http"

	"github.com/gbrlsnchs/jwt/v3"
)

const APIErrCode = 1

type APIResult struct {
	ErrCode int
	ErrMsg  string
	Data    interface{}
}

// Define a custom multiplexer type
type Server struct {
	routes map[string]http.Handler
}

func NewServer(config *Config) (*Server, error) {
	redis := redis.NewRedis(config.RedisAddr, config.RedisPass)
	if config.PrivateKey == "" {
		return nil, jwt.ErrHMACMissingKey
	}

	handler := newServerHandler(config, newDevMgr(context.Background(), redis), redis, jwt.NewHS256([]byte(config.PrivateKey)))

	s := &Server{routes: make(map[string]http.Handler)}
	// /update/lua support old agent
	s.handle("/update/lua", http.HandlerFunc(handler.handleLuaUpdate))
	s.handle("/config/lua", http.HandlerFunc(handler.handleGetLuaConfig))
	s.handle("/config/controller", http.HandlerFunc(handler.handleGetControllerConfig))
	s.handle("/config/apps", handler.auth.proxy(handler.handleGetAppsConfig))
	s.handle("/config/ssh", handler.auth.proxy(handler.handleGetWssshConfig))
	// s.handle("/ws", handler.auth.proxy(handler.handleGetWssshConfig))

	s.handle("/rds/eval", handler.auth.proxy(handler.handleRdsEval))

	// create ws server and upgrade client
	s.handle("/agent/list", http.HandlerFunc(handler.handleAgentList))
	s.handle("/controller/list", http.HandlerFunc(handler.handleControllerList))

	s.handle("/api/applist", http.HandlerFunc(handler.handleGetAppList))
	s.handle("/api/appinfo", http.HandlerFunc(handler.handleGetAppInfo))
	s.handle("/api/signverify", http.HandlerFunc(handler.handleSignVerify))
	s.handle("/api/nodeinfo", http.HandlerFunc(handler.handleGetNodeInfo))
	s.handle("/api/nodelist", http.HandlerFunc(handler.handleGetNodeList))
	s.handle("/api/appinfos", http.HandlerFunc(handler.handleGetAllNodesAppInfosList))
	s.handle("/api/nodeip", http.HandlerFunc(handler.handleLoadNodeIP))
	s.handle("/api/clearip", http.HandlerFunc(handler.handleClearIP))
	s.handle("/api/health", http.HandlerFunc(handler.HealthCheck))
	s.handle("/api/onlineDuration", http.HandlerFunc(handler.handleOnlineDurationByDate))
	s.handle("/api/setNodeConfigs", http.HandlerFunc(handler.handleSetNodeConfigs))
	s.handle("/api/getNodeConfigs", http.HandlerFunc(handler.handleGetNodeConfigs))

	s.handle("/push/metrics", handler.auth.proxy(handler.handlePushMetrics))
	s.handle("/push/appinfo", handler.auth.proxy(handler.handlePushAppInfo))

	s.handle("/node/regist", http.HandlerFunc(handler.HandleNodeRegist))
	s.handle("/node/registWithWallet", http.HandlerFunc(handler.HandleNodeRegistWithWallet))
	s.handle("/node/login", http.HandlerFunc(handler.HandleNodeLogin))
	s.handle("/node/keepalive", handler.auth.proxy(handler.HandleNodeKeepalive))
	s.handle("/node/getConfig", handler.auth.proxy(handler.HandleNodeGetConfig))

	s.handle("/node/next/id", http.HandlerFunc(handler.HandleNextId))
	return s, nil
}

// Implement the ServeHTTP method for CustomServeMux
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, found := s.routes[r.URL.Path]
	if found {
		handler.ServeHTTP(w, r)
	} else {
		http.DefaultServeMux.ServeHTTP(w, r)
	}
}

// Register a route with the custom multiplexer
func (s *Server) handle(pattern string, handler http.Handler) {
	s.routes[pattern] = handler
}
