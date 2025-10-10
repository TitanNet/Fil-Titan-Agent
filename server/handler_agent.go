package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

func (h *ServerHandler) handleLuaUpdate(w http.ResponseWriter, r *http.Request) {
	log.Infof("handleLuaUpdate, queryString %s\n", r.URL.RawQuery)

	d := NewDeviceFromURLQuery(r.URL.Query())
	if d != nil {
		recordRawIP(d.UUID, r)
		ip := getClientIP(r)
		if ip != "" {
			d.IP = ip
		}

		h.devMgr.updateAgent(&Agent{*d})
	}

	os := r.URL.Query().Get("os")
	uuid := r.URL.Query().Get("uuid")

	var testScripName string
	testNode := h.config.TestNodes[uuid]
	if testNode != nil {
		testScripName = testNode.LuaScript
	}

	// log.Printf("testNode %#v", testNode)
	var file *FileConfig = nil
	for _, f := range h.config.LuaFileList {
		if len(testScripName) > 0 {
			if f.Name == testScripName {
				file = f
				break
			}
		} else if f.OS == os {
			file = f
			break
		}
	}

	if file == nil {
		resultError(w, http.StatusBadRequest, fmt.Sprintf("can not find the os %s script", os))
		return
	}

	buf, err := json.Marshal(file)
	if err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Write(buf)
}

func (h *ServerHandler) handleGetLuaConfig(w http.ResponseWriter, r *http.Request) {
	h.handleLuaUpdate(w, r)
}

func (h *ServerHandler) handleGetControllerConfig(w http.ResponseWriter, r *http.Request) {
	log.Infof("handleGetControllerConfig, queryString %s\n", r.URL.RawQuery)
	// version := r.URL.Query().Get("version")
	os := r.URL.Query().Get("os")
	uuid := r.URL.Query().Get("uuid")
	arch := r.URL.Query().Get("arch")
	isBox, _ := strconv.ParseBool(r.URL.Query().Get("isBox"))

	controllerStr, err := h.redis.GetNodeSpecifiedController(r.Context(), uuid)
	if err != nil {
		log.Errorf("GetNodeSpecifiedController error: %v", err)
	}
	if controllerStr != "" {
		var specifiedController *FileConfig
		if err = json.Unmarshal([]byte(controllerStr), &specifiedController); err != nil {
			log.Errorf("json.Unmarshal error: %v", err)
		}
		if specifiedController != nil {
			if err = json.NewEncoder(w).Encode(specifiedController); err != nil {
				log.Errorf("json.NewEncoder error: %v", err)
			}
			return
		}
	}

	var testControllerName string
	testNode := h.config.TestNodes[uuid]
	if testNode != nil {
		testControllerName = testNode.Controller
	}

	var file *FileConfig = nil
	var bestMatchFile *FileConfig = nil

	for _, f := range h.config.ControllerFileList {
		if len(testControllerName) > 0 {
			if f.Name == testControllerName {
				file = f
				break
			}
		}
		if f.OS == os {
			// common version
			if f.Tag == "" && file == nil {
				file = f
				// arch match version
			} else if f.Tag != "" && arch != "" && strings.Contains(f.Tag, arch) {
				bestMatchFile = f
				break
			} else if isBox && f.Tag == "box" {
				bestMatchFile = f
				break
			}
		}
	}

	var finalFile *FileConfig = file
	if bestMatchFile != nil {
		finalFile = bestMatchFile
	}

	if finalFile == nil {
		resultError(w, http.StatusBadRequest, fmt.Sprintf("can not find the os %s", os))
		return
	}

	buf, err := json.Marshal(finalFile)
	if err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Write(buf)
}
