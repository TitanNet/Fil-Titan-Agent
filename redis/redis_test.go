package redis

import (
	"agent/common"
	titanrsa "agent/common/rsa"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"log"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/google/uuid"
)

const (
	redisAddr = "127.0.0.1:6379"
)

func TestNode(t *testing.T) {
	t.Logf("TestNode")

	redis := NewRedis(redisAddr, "")

	node := Node{
		ID:       uuid.NewString(),
		OS:       "windows",
		Platform: "10.01",
	}
	err := redis.SetNode(context.Background(), &node)
	if err != nil {
		t.Fatal("set node err:", err.Error())
	}

	n, err := redis.GetNode(context.Background(), node.ID)
	if err != nil {
		t.Fatal("set node err:", err.Error())
	}

	t.Logf("node:%#v", *n)
}

func TestAppItem(t *testing.T) {
	t.Logf("TestApp")

	redis := NewRedis(redisAddr, "")

	app := App{
		AppName: "titan-l2",
		// relative app dir
		AppDir:     "/opt/titan/apps/titan-l2",
		ScriptName: "titan-l2.lua",
		ScriptMD5:  "585f384ec97532e9822b7863ddeb958a",
		Version:    "0.0.1",
		ScriptURL:  "https://agent.titannet.io/titan-l2.lua",
	}
	err := redis.SetApp(context.Background(), &app)
	if err != nil {
		t.Fatal("set app err:", err.Error())
	}

	titanL2App, err := redis.GetApp(context.Background(), app.AppName)
	if err != nil {
		t.Fatal("get app err:", err.Error())
	}

	t.Logf("app: %#v", titanL2App)
}

func TestApps(t *testing.T) {
	t.Logf("TestApps")

	redis := NewRedis(redisAddr, "")

	app1 := App{
		AppName: "titan-l2",
		// relative app dir
		AppDir:     "/opt/titan/apps/titan-l2",
		ScriptName: "titan-l2.lua",
		ScriptMD5:  "585f384ec97532e9822b7863ddeb958a",
		Version:    "0.0.1",
		ScriptURL:  "https://agent.titannet.io/titan-l2.lua",
	}

	app2 := App{
		AppName: "titan-l1",
		// relative app dir
		AppDir:     "/opt/titan/apps/titan-l1",
		ScriptName: "titan-l1.lua",
		ScriptMD5:  "585f384ec97532e9822b7863ddeb958a",
		Version:    "0.0.1",
		ScriptURL:  "https://agent.titannet.io/titan-l1.lua",
	}
	err := redis.SetApps(context.Background(), []*App{&app1, &app2})
	if err != nil {
		t.Fatal("set app err:", err.Error())
	}

	apps, err := redis.GetApps(context.Background(), []string{app1.AppName, app2.AppName})
	if err != nil {
		t.Fatal("get app err:", err.Error())
	}

	t.Log("apps:")
	for _, app := range apps {
		t.Logf("%#v", app)
	}
}

func TestNodeApp(t *testing.T) {
	t.Logf("TestNodeApp")

	redis := NewRedis(redisAddr, "")

	app := NodeApp{
		AppName: "titan-l2",
		MD5:     "585f384ec97532e9822b7863ddeb958a",
		Metric:  "abc",
	}

	nodeID := uuid.NewString()

	err := redis.SetNodeApp(context.Background(), nodeID, &app)
	if err != nil {
		t.Fatal("set app err:", err.Error())
	}

	titanL2App, err := redis.GetNodeApp(context.Background(), nodeID, app.AppName)
	if err != nil {
		t.Fatal("get app err:", err.Error())
	}

	t.Logf("app: %#v", titanL2App)
}

func TestNodeApps(t *testing.T) {
	t.Logf("TestNodeApps")

	redis := NewRedis(redisAddr, "")

	app1 := NodeApp{
		AppName: "titan-l2",
		MD5:     "585f384ec97532e9822b7863ddeb958a",
		Metric:  "abc",
	}

	app2 := NodeApp{
		AppName: "titan-l1",
		MD5:     "585f384ec97532e9822b7863ddeb958a",
		Metric:  "abc",
	}

	nodeID := uuid.NewString()

	err := redis.SetNodeApps(context.Background(), nodeID, []*NodeApp{&app1, &app2})
	if err != nil {
		t.Fatal("set app err:", err.Error())
	}

	nodeApps, err := redis.GetNodeApps(context.Background(), nodeID, []string{app1.AppName, app2.AppName})
	if err != nil {
		t.Fatal("get app err:", err.Error())
	}

	t.Log("node apps:")
	for _, nodeApp := range nodeApps {
		t.Logf("%#v", nodeApp)
	}
}

func TestNodeAppList(t *testing.T) {
	t.Logf("TestNodeApps")

	redis := NewRedis(redisAddr, "")

	nodeID := uuid.NewString()

	err := redis.AddNodeAppsToList(context.Background(), nodeID, []string{"titan-l1", "titan-l2"})
	if err != nil {
		t.Fatal("set app err:", err.Error())
	}

	appList, err := redis.GetNodeAppList(context.Background(), nodeID)
	if err != nil {
		t.Fatal("get app err:", err.Error())
	}

	t.Logf("appList:%#v", appList)
}

func TestDeleteNodeAppList(t *testing.T) {
	// t.Logf("TestNodeApps")

	// redis := NewRedis(redisAddr, "")

	// nodeID := "52b67296-940c-434e-85f3-16df4aa9c6ed"

	// err := redis.DeleteNodeApps(context.Background(), nodeID, []string{"titan-l1", "titan-l2"})
	// if err != nil {
	// 	t.Fatal("delete app list err:", err.Error())
	// }

	fmt.Println(net.SplitHostPort("https://www.google.com:443"))

}

func TestPriKey(t *testing.T) {
	keyPath := "/Users/zt/private.key"
	nodeid := "a4d7ef41-4316-4288-952a-b09111c0fbaf"
	serverUrl := "https://test4-api.titannet.io"

	bytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(bytes), err)

	priKey, err := titanrsa.Pem2PrivateKey(bytes)
	if err != nil {
		t.Fatal(err)
	}

	// t.Log(priKey)

	rsa := titanrsa.New(crypto.SHA256, crypto.SHA256.New())
	sign, err := rsa.Sign(priKey, []byte(nodeid))
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf("%s%s?node_id=%s&sign=%s", serverUrl, "/node/login", nodeid, hex.EncodeToString(sign))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("regist status code: %d, msg: %s, url: %s", resp.StatusCode, string(body), url)
		t.Fatalf("regist status code: %d", resp.StatusCode)
	}

	t.Log(resp.Header.Get("Web-Server"))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(body))
}

func TestLogin(t *testing.T) {

	// priKey, _ := titanrsa.GeneratePrivateKey(1024)
	// fmt.Println(string(titanrsa.PrivateKey2Pem(priKey)))

	payload := common.JwtPayload{
		NodeID: "c19bd471-7a11-4bd4-bec5-a515fd3d4ff6",
	}

	config, err := ParseConfig("./server.yaml")
	if err != nil {
		panic(err)
	}

	tk, err := jwt.Sign(payload, jwt.NewHS256([]byte(config.PrivateKey)))
	fmt.Println(string(tk), err)

}

func TestLoginFromLocal(t *testing.T) {
	bytes, err := os.ReadFile("private_key444.pem")
	if err != nil {
		t.Fatal(err.Error())
	}

	priKey, err := titanrsa.Pem2PrivateKey(bytes)

	rsa := titanrsa.New(crypto.SHA256, crypto.SHA256.New())
	sign, err := rsa.Sign(priKey, []byte("7a71b485-3c14-453f-b134-6eb56f74b664"))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("sign=", hex.EncodeToString(sign))

}

func TestParse(t *testing.T) {
	pubkey := []byte("LS0tLS1CRUdJTiBSU0EgUFVCTElDIEtFWS0tLS0tCk1JR0pBb0dCQUwzZHp3VllzQ3ROMlg0dk9aK2l1RUtzS2J6ZVZ5a0FpRWtBRTRoU1k0anJ4blBHbjcvNmpxRmEKeU5HVVdJQWV6V3U2dGJBSnhINDVISGVFNFhyTG42QzNUL09heG1YMEtNWjZjMzhLVm5IQVVYSnBKeS8ycXpFRworR3pmTHhrQWNucmdVanlucW92amk3Z2txVGpNYkNSZ2FyeEltcXorTXNDYnNiT0QrQ1ZOQWdNQkFBRT0KLS0tLS1FTkQgUlNBIFBVQkxJQyBLRVktLS0tLQ%3D%3D")

	pubKeyBytes, err := base64.URLEncoding.DecodeString(string(pubkey))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := titanrsa.Pem2PublicKey(pubKeyBytes); err != nil {
		t.Fatal(err)
	}

}
func TestCheckRegex(t *testing.T) {
	v := "TT202502011LJLFSVLEA"

	fmt.Println(regexp.MustCompile(`^TT(\d{4})(\d{2})(\d{2})([A-Za-z0-9]{10})$`).MatchString(v))
}
