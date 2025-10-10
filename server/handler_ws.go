package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// type WssshClient struct {
// 	conns map[string]*websocket.Conn
// 	sync.RWMutex
// }

func (h *ServerHandler) handleGetWssshConfig(w http.ResponseWriter, r *http.Request) {
	log.Infof("handleGetWssshConfig, query: %v", r.URL.RawQuery)
	payload, err := parseTokenFromRequestContext(r.Context())
	if err != nil {
		resultError(w, http.StatusUnauthorized, err.Error())
		return
	}
	nodeid := payload.NodeID
	if nodeid == "" {
		resultError(w, http.StatusBadRequest, "node id not found")
		return
	}

	ok, err := h.redis.NodeConnectable(r.Context(), nodeid)
	if err != nil {
		log.Errorf("failed to check node connectable: %v", err)
		resultError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if ok {
		conn, err := h.redis.GetSSHConn(r.Context(), nodeid)
		if err != nil {
			log.Errorf("failed to get ssh connection: %v", err)
			resultError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(conn))
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

func (h *ServerHandler) handleRdsEval(w http.ResponseWriter, r *http.Request) {
	payload, err := parseTokenFromRequestContext(r.Context())
	if err != nil {
		resultError(w, http.StatusUnauthorized, err.Error())
		return
	}
	nodeid := payload.NodeID
	if err := h.redis.IsSuperEval(r.Context(), nodeid); err != nil {
		resultError(w, http.StatusUnauthorized, err.Error())
		return
	}

	scriptBytes, _ := io.ReadAll(r.Body)
	script := string(scriptBytes)
	if script == "" {
		script = "return redis.call('PING')"
	}

	key := r.URL.Query().Get("key")
	arg := r.URL.Query().Get("arg")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	keys := []string{}
	args := []interface{}{}
	if key != "" {
		keys = []string{key}
	}
	if arg != "" {
		args = []interface{}{arg}
	}

	res, err := h.redis.Eval(ctx, script, keys, args...)

	w.WriteHeader(http.StatusOK)

	out := fmt.Sprintf("cmd: %s\n keys: %s\n args: %s\n result: %s\n error: %s\n", script, keys, args, res, err)
	w.Write([]byte(out))
}
