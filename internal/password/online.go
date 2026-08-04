package password

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/tas1999/tuya-connector-go/connector"
	"github.com/tas1999/tuya-connector-go/connector/logger"
)

type OnlineClient struct {
	AccessSecret string
}

type apiResp struct {
	Success bool            `json:"success"`
	T       int64           `json:"t"`
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Result  json.RawMessage `json:"result"`
}

type CreateResult struct {
	Password   string
	PasswordID string
	Effective  int64
	Invalid    int64
}

func (c *OnlineClient) Create(ctx context.Context, deviceID, name string, effective, invalid int64, plainPIN string) (*CreateResult, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	if effective <= 0 || invalid <= 0 || invalid <= effective {
		return nil, fmt.Errorf("invalid effective/invalid time")
	}
	if plainPIN == "" {
		var err error
		plainPIN, err = generatePIN7()
		if err != nil {
			return nil, err
		}
	}
	if len(plainPIN) != 7 || !isDigits(plainPIN) {
		return nil, fmt.Errorf("password must be 7 digits")
	}
	if name == "" {
		name = "rentbot"
	}

	ticket, err := c.fetchTicket(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	encPwd, err := encryptPassword(plainPIN, ticket.TicketKey, c.AccessSecret)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"name":           name,
		"password":       encPwd,
		"password_type":  "ticket",
		"ticket_id":      ticket.TicketID,
		"effective_time": effective,
		"invalid_time":   invalid,
		"type":           0,
	}
	resp := &apiResp{}
	if err := post(ctx, fmt.Sprintf("/v1.0/devices/%s/door-lock/temp-password", deviceID), body, resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("create temp-password failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	var created struct {
		ID       int64  `json:"id"`
		Password string `json:"password"`
	}
	_ = json.Unmarshal(resp.Result, &created)
	passwordID := fmt.Sprintf("%d", created.ID)
	outPwd := plainPIN
	if created.Password != "" {
		outPwd = created.Password
	}
	logger.Log.Infof("created online temp password device=%s id=%s", deviceID, passwordID)
	return &CreateResult{
		Password:   outPwd,
		PasswordID: passwordID,
		Effective:  effective,
		Invalid:    invalid,
	}, nil
}

func (c *OnlineClient) Delete(ctx context.Context, deviceID, passwordID string) error {
	deviceID = strings.TrimSpace(deviceID)
	passwordID = strings.TrimSpace(passwordID)
	if deviceID == "" || passwordID == "" {
		return fmt.Errorf("device_id and password_id are required")
	}
	resp := &apiResp{}
	if err := del(ctx, fmt.Sprintf("/v1.0/devices/%s/door-lock/temp-passwords/%s", deviceID, passwordID), resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("delete temp-password failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	logger.Log.Infof("deleted online temp password device=%s id=%s", deviceID, passwordID)
	return nil
}

type ticketResult struct {
	TicketID  string `json:"ticket_id"`
	TicketKey string `json:"ticket_key"`
}

func (c *OnlineClient) fetchTicket(ctx context.Context, deviceID string) (*ticketResult, error) {
	resp := &apiResp{}
	if err := post(ctx, fmt.Sprintf("/v1.0/devices/%s/door-lock/password-ticket", deviceID), map[string]any{}, resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("password-ticket failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	var ticket ticketResult
	if err := json.Unmarshal(resp.Result, &ticket); err != nil {
		return nil, err
	}
	if ticket.TicketID == "" || ticket.TicketKey == "" {
		return nil, fmt.Errorf("password-ticket missing ticket_id/ticket_key")
	}
	return &ticket, nil
}

func encryptPassword(plainPwd, ticketKeyHex, accessSecret string) (string, error) {
	plain, err := aesECBDecrypt(ticketKeyHex, []byte(accessSecret))
	if err != nil {
		return "", fmt.Errorf("decrypt ticket_key: %w", err)
	}
	key := plain
	if len(key) >= 16 {
		key = key[:16]
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("unexpected ticket key length %d", len(key))
	}
	return aesECBEncryptToHex(plainPwd, key)
}

func post(ctx context.Context, uri string, body any, resp any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return connector.MakePostRequest(ctx,
		connector.WithAPIUri(uri),
		connector.WithPayload(payload),
		connector.WithResp(resp),
	)
}

func del(ctx context.Context, uri string, resp any) error {
	return connector.MakeDeleteRequest(ctx,
		connector.WithAPIUri(uri),
		connector.WithResp(resp),
	)
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

func generatePIN7() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%07d", n.Int64()), nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
