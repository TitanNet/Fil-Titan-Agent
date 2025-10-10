package server

import (
	"agent/common"
	titanrsa "agent/common/rsa"
	"agent/redis"

	"bytes"
	"context"
	"crypto"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"crypto/rsa"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	log "github.com/sirupsen/logrus"
)

type ServerHandler struct {
	config *Config
	devMgr *DevMgr
	redis  *redis.Redis
	auth   *auth
	// authenticate func
}

// type tokenPayload struct {
// }

type auth struct {
	apiSecret *jwt.HMACSHA
}

func (a *auth) proxy(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")

		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var payload common.JwtPayload
		if _, err := jwt.Verify([]byte(token), a.apiSecret, &payload); err != nil {
			log.Errorf("jwt.Verify: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), "payload", payload))

		// curlCmd, err := RequestToCurl(r)
		// if err == nil {
		// 	log.Infof("RequestToCurl: %s", curlCmd)
		// }

		next(w, r)
	}
}

func RequestToCurl(req *http.Request) (string, error) {
	var curlCmd strings.Builder

	curlCmd.WriteString("curl -X ")
	curlCmd.WriteString(req.Method)

	for key, values := range req.Header {
		for _, value := range values {
			curlCmd.WriteString(fmt.Sprintf(" -H '%s: %s'", key, value))
		}
	}

	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read request body: %w", err)
		}

		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if len(bodyBytes) > 0 {
			curlCmd.WriteString(fmt.Sprintf(" -d '%s'", string(bodyBytes)))
		}
	}

	curlCmd.WriteString(fmt.Sprintf(" '%s'", req.URL.String()))

	return curlCmd.String(), nil
}

func parseTokenFromRequestContext(ctx context.Context) (*common.JwtPayload, error) {
	payload, ok := ctx.Value("payload").(common.JwtPayload)
	if !ok {
		return nil, fmt.Errorf("no payload in context")
	}
	return &payload, nil
}

func (a *auth) sign(p common.JwtPayload) ([]byte, error) {
	return jwt.Sign(p, a.apiSecret)
}

func newServerHandler(config *Config, devMgr *DevMgr, redis *redis.Redis, authApiSecret *jwt.HMACSHA) *ServerHandler {
	return &ServerHandler{config: config, devMgr: devMgr, redis: redis, auth: &auth{apiSecret: authApiSecret}}
}

func (h *ServerHandler) handleAgentList(w http.ResponseWriter, r *http.Request) {
	log.Infof("handleAgentList, queryString %s\n", r.URL.RawQuery)

	agents := h.devMgr.getAgents()

	result := struct {
		Total  int      `json:"total"`
		Agents []*Agent `json:"agents"`
	}{
		Total:  len(agents),
		Agents: agents,
	}

	formattedJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		http.Error(w, "Failed to format JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(formattedJSON)
}
func (h *ServerHandler) HandleNextId(w http.ResponseWriter, r *http.Request) {
	nextId, err := h.redis.GetSNNextID(r.Context())
	if err != nil {
		resultError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := json.NewEncoder(w).Encode(APIResult{Data: map[string]string{
		"sn": nextId,
	}}); err != nil {
		log.Error("ServerHandler.handleSignVerify, Encode: ", err.Error())
	}
}

func resultError(w http.ResponseWriter, statusCode int, errMsg string) {
	w.WriteHeader(statusCode)
	w.Write([]byte(errMsg))
}

func apiResultErr(w http.ResponseWriter, errMsg string) {
	result := APIResult{ErrCode: APIErrCode, ErrMsg: errMsg}
	buf, err := json.Marshal(result)
	if err != nil {
		log.Error("apiResult, Marshal: ", err.Error())
		return
	}

	if _, err := w.Write(buf); err != nil {
		log.Error("apiResult, Write: ", err.Error())
	}
}

// cpu/number memory/MB disk/GB
func getResource(r *http.Request) (os string, cpu int, memory int64, disk int64, arch string) {
	os = r.URL.Query().Get("os")

	cpuStr := r.URL.Query().Get("cpu")
	memoryStr := r.URL.Query().Get("memory")
	diskStr := r.URL.Query().Get("disk")

	cpu = stringToInt(cpuStr)

	memoryBytes := stringToInt64(memoryStr)
	memory = memoryBytes / (1024 * 1024)

	diskBytes := stringToInt64(diskStr)
	disk = diskBytes / (1024 * 1024 * 1024)
	arch = r.URL.Query().Get("arch")
	return
}

func verifySignatureWithRsaPubKey(pubKey string, msg, signatrue []byte) error {
	pub, err := titanrsa.Pem2PublicKey([]byte(pubKey))
	if err != nil {
		return fmt.Errorf("verifySignatureWithRsaPubKey, Parse rsa public key  : %s", err.Error())
	}

	hash := crypto.SHA256.New()
	_, err = hash.Write(msg)
	if err != nil {
		return fmt.Errorf("verifySignatureWithRsaPubKey, hash write failed: %s", err.Error())
	}
	sum := hash.Sum(nil)

	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum, signatrue); err != nil {
		return fmt.Errorf("verifySignatureWithRsaPubKey, VerifyPKCS1v15 failed: %s", err.Error())
	}

	return nil
}

func verifySignatureWithSecp256k1PubKey(pubKey string, msg, signatrue []byte) error {
	log.Infof("pub key:%s", pubKey)
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return err
	}

	var newPubKey = secp256k1.PubKey(pubKeyBytes)
	if !newPubKey.VerifySignature(msg, signatrue) {
		return fmt.Errorf("verifySignatureWithSecp256k1PubKey, VerifySignature failed")
	}

	return nil
}

func verifySignature(registerInfo *redis.NodeRegistInfo, msg, signatrue []byte) error {
	if len(registerInfo.Secp256k1PublicKey) > 0 {
		return verifySignatureWithSecp256k1PubKey(registerInfo.Secp256k1PublicKey, msg, signatrue)
	}

	if len(registerInfo.PublicKey) > 0 {
		return verifySignatureWithRsaPubKey(registerInfo.PublicKey, msg, signatrue)
	}

	return fmt.Errorf("verifySignature, public key can not empty")
}
