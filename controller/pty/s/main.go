package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

//go:embed index.html
var indexHtml embed.FS

var agents = make(map[string]*websocket.Conn)
var lock = sync.RWMutex{}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func agentWS(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("nodeId")
	// token := r.URL.Query().Get("token")
	if nodeID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("upgrade error: %v", err)
		return
	}

	lock.Lock()
	agents[nodeID] = conn
	lock.Unlock()

	log.Println("Agent connected:", nodeID)
}

func frontendWS(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("nodeId")

	lock.RLock()
	agentConn, ok := agents[nodeID]
	lock.RUnlock()

	if !ok {
		http.Error(w, "agent not online", http.StatusNotFound)
		return
	}

	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("client upgrade error: %v", err)
		return
	}

	go func() {
		// defer func() {
		// 	clientConn.Close()
		// 	lock.Lock()
		// 	delete(agents, nodeID)
		// 	lock.Unlock()
		// 	log.Println("Agent disconnected:", nodeID)
		// }()

		for {
			_, msg, err := clientConn.ReadMessage()
			if err != nil {

				lock.Lock()
				agentConn.Close()
				delete(agents, nodeID)
				lock.Unlock()

				log.Errorf("read message error: %v", err)
				return
			}
			agentConn.WriteMessage(websocket.BinaryMessage, msg)
		}
	}()

	go func() {
		// defer clientConn.Close()
		for {
			_, msg, err := agentConn.ReadMessage()
			if err != nil {
				log.Errorf("read message error: %v", err)
				return
			}
			clientConn.WriteMessage(websocket.BinaryMessage, msg)
		}
	}()
}

func listAgents(w http.ResponseWriter, r *http.Request) {
	lock.RLock()
	var ids []string
	for id := range agents {
		ids = append(ids, id)
	}
	lock.RUnlock()

	sort.Strings(ids)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ids)
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html, err := indexHtml.ReadFile("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(html)
	})
	http.HandleFunc("/agent/ws", agentWS)
	http.HandleFunc("/frontend/ws", frontendWS)
	http.HandleFunc("/agents", listAgents)

	fmt.Println("Server running on :6866")
	log.Fatal(http.ListenAndServe(":6866", nil))
}
