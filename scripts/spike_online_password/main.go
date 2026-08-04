// Spike: online temp password create + DELETE (remote revoke).
package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/tas1999/tuya-connector-go/connector"
	"github.com/tas1999/tuya-connector-go/connector/env"
	"github.com/tas1999/tuya-connector-go/connector/httplib"
)

const deviceID = "bf22adc2ea2e5d66bdwdcc"

type apiResp struct {
	Success bool            `json:"success"`
	T       int64           `json:"t"`
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Result  json.RawMessage `json:"result"`
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty")
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > len(data) {
		return nil, fmt.Errorf("bad pad %d", pad)
	}
	return data[:len(data)-pad], nil
}

func aesECBDecrypt(cipherHex string, key []byte) ([]byte, error) {
	raw, err := hex.DecodeString(cipherHex)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(raw)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("cipher len %d not multiple of block", len(raw))
	}
	out := make([]byte, len(raw))
	for i := 0; i < len(raw); i += block.BlockSize() {
		block.Decrypt(out[i:i+block.BlockSize()], raw[i:i+block.BlockSize()])
	}
	return pkcs7Unpad(out)
}

func aesECBEncryptToHex(plain string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad([]byte(plain), block.BlockSize())
	out := make([]byte, len(padded))
	for i := 0; i < len(padded); i += block.BlockSize() {
		block.Encrypt(out[i:i+block.BlockSize()], padded[i:i+block.BlockSize()])
	}
	return hex.EncodeToString(out), nil
}

func post(uri string, body any, resp any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	opts := []connector.ParamFunc{connector.WithAPIUri(uri), connector.WithResp(resp)}
	if payload != nil {
		opts = append(opts, connector.WithPayload(payload))
	}
	return connector.MakePostRequest(context.Background(), opts...)
}

func del(uri string, resp any) error {
	return connector.MakeDeleteRequest(context.Background(),
		connector.WithAPIUri(uri),
		connector.WithResp(resp),
	)
}

func get(uri string, resp any) error {
	return connector.MakeGetRequest(context.Background(),
		connector.WithAPIUri(uri),
		connector.WithResp(resp),
	)
}

func printJSON(label string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(label)
	fmt.Println(string(b))
}

func tryEncryptPassword(plainPwd, ticketKeyHex, accessSecret string) (enc string, method string, err error) {
	type attempt struct {
		name string
		key  []byte
	}
	var attempts []attempt

	// decrypt ticket_key with access secret variants
	secretASCII := []byte(accessSecret)
	secretHex, hexErr := hex.DecodeString(accessSecret)

	decryptKeys := []struct {
		name string
		key  []byte
	}{
		{"secretASCII32-AES256", secretASCII},
	}
	if len(secretASCII) >= 16 {
		decryptKeys = append(decryptKeys, struct {
			name string
			key  []byte
		}{"secretASCII16-AES128", secretASCII[:16]})
	}
	if hexErr == nil && len(secretHex) == 16 {
		decryptKeys = append(decryptKeys, struct {
			name string
			key  []byte
		}{"secretHex16-AES128", secretHex})
	}

	for _, dk := range decryptKeys {
		plain, derr := aesECBDecrypt(ticketKeyHex, dk.key)
		if derr != nil {
			fmt.Printf("decrypt fail %s: %v\n", dk.name, derr)
			continue
		}
		fmt.Printf("decrypt ok %s: plain_len=%d plain_hex=%s ascii=%q\n",
			dk.name, len(plain), hex.EncodeToString(plain), string(plain))

		// password encrypt key candidates from decrypted ticket
		candidates := []attempt{
			{dk.name + "+raw", plain},
		}
		if len(plain) >= 16 {
			candidates = append(candidates, attempt{dk.name + "+raw16", plain[:16]})
		}
		if decoded, e := hex.DecodeString(string(plain)); e == nil && (len(decoded) == 16 || len(decoded) == 32) {
			candidates = append(candidates, attempt{dk.name + "+asHexString", decoded})
			if len(decoded) >= 16 {
				candidates = append(candidates, attempt{dk.name + "+asHexString16", decoded[:16]})
			}
		}
		attempts = append(attempts, candidates...)
	}

	for _, a := range attempts {
		if len(a.key) != 16 && len(a.key) != 24 && len(a.key) != 32 {
			continue
		}
		enc, eerr := aesECBEncryptToHex(plainPwd, a.key)
		if eerr != nil {
			continue
		}
		// Prefer first successful crypto construction; API will validate
		return enc, a.name, nil
	}
	return "", "", fmt.Errorf("no decrypt/encrypt combination worked")
}

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	accessSecret := os.Getenv("TUYA_ACCESSKEY")
	if accessSecret == "" {
		fmt.Println("TUYA_ACCESSKEY empty")
		os.Exit(1)
	}

	connector.InitWithOptions(
		env.WithApiHost(httplib.URL_EU),
		env.WithMsgHost(httplib.MSG_EU),
		env.WithAppName("tuyaSDK"),
		env.WithDebugMode(true),
	)

	now := time.Now()
	effective := now.Truncate(time.Hour).Unix()
	invalid := now.Truncate(time.Hour).Add(2 * time.Hour).Unix()
	plainPwd := "1357924" // 7 digits for Wi-Fi lock

	fmt.Println("=== 1) password-ticket ===")
	ticketResp := &apiResp{}
	if err := post(fmt.Sprintf("/v1.0/devices/%s/door-lock/password-ticket", deviceID), map[string]any{}, ticketResp); err != nil {
		fmt.Println("ticket transport:", err)
	}
	printJSON("ticket raw:", ticketResp)
	if !ticketResp.Success {
		fmt.Println("ticket failed")
		os.Exit(1)
	}
	var ticket struct {
		TicketID   string `json:"ticket_id"`
		TicketKey  string `json:"ticket_key"`
		ExpireTime int64  `json:"expire_time"`
	}
	_ = json.Unmarshal(ticketResp.Result, &ticket)

	encPwd, method, err := tryEncryptPassword(plainPwd, ticket.TicketKey, accessSecret)
	if err != nil {
		fmt.Println("encrypt:", err)
		os.Exit(1)
	}
	fmt.Printf("encrypt method=%s enc_len=%d\n", method, len(encPwd))

	fmt.Println("=== 2) CREATE online temp-password ===")
	createBody := map[string]any{
		"name":           "spike-online-test",
		"password":       encPwd,
		"password_type":  "ticket",
		"ticket_id":      ticket.TicketID,
		"effective_time": effective,
		"invalid_time":   invalid,
		"type":           0,
	}
	createResp := &apiResp{}
	_ = post(fmt.Sprintf("/v1.0/devices/%s/door-lock/temp-password", deviceID), createBody, createResp)
	printJSON("create:", createResp)

	if !createResp.Success {
		fmt.Println("create failed — try v2 unnamed")
		createResp = &apiResp{}
		_ = post(fmt.Sprintf("/v2.0/devices/%s/door-lock/temp-password", deviceID), createBody, createResp)
		printJSON("create v2:", createResp)
	}
	if !createResp.Success {
		os.Exit(1)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(createResp.Result, &created)
	fmt.Println("created password_id=", created.ID, "plain=", plainPwd)

	fmt.Println("=== 3) LIST temp-passwords ===")
	listResp := &apiResp{}
	_ = get(fmt.Sprintf("/v1.0/devices/%s/door-lock/temp-passwords?valid=true", deviceID), listResp)
	printJSON("list before delete:", listResp)

	fmt.Println("=== 4) DELETE temp-passwords/", created.ID, "===")
	delResp := &apiResp{}
	_ = del(fmt.Sprintf("/v1.0/devices/%s/door-lock/temp-passwords/%d", deviceID, created.ID), delResp)
	printJSON("delete:", delResp)

	fmt.Println("=== 5) LIST after delete ===")
	list2 := &apiResp{}
	_ = get(fmt.Sprintf("/v1.0/devices/%s/door-lock/temp-passwords?valid=true", deviceID), list2)
	printJSON("list after delete:", list2)

	fmt.Println("=== SUMMARY ===")
	fmt.Printf("create success=%v id=%d\n", createResp.Success, created.ID)
	fmt.Printf("delete success=%v code=%d msg=%q result=%s\n", delResp.Success, delResp.Code, delResp.Msg, string(delResp.Result))
}
