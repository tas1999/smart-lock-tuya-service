// One-off spike: create offline temp password, then clear_one. Not part of production.
package main

import (
	"context"
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

type tuyaResp struct {
	Success bool  `json:"success"`
	T       int64 `json:"t"`
	Result  struct {
		EffectiveTime           int    `json:"effective_time"`
		InvalidTime             int    `json:"invalid_time"`
		OfflineTempPassword     string `json:"offline_temp_password"`
		OfflineTempPasswordID   string `json:"offline_temp_password_id"`
		OfflineTempPasswordName string `json:"offline_temp_password_name"`
	} `json:"result"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func hourUnix(t time.Time) int64 {
	t = t.Truncate(time.Hour)
	return t.Unix()
}

func postOffline(body map[string]any) (*tuyaResp, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp := &tuyaResp{}
	err = connector.MakePostRequest(
		context.Background(),
		connector.WithAPIUri(fmt.Sprintf("/v1.1/devices/%s/door-lock/offline-temp-password", deviceID)),
		connector.WithPayload(payload),
		connector.WithResp(resp),
	)
	return resp, err
}

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	if os.Getenv("TUYA_ACCESSID") == "" {
		fmt.Println("TUYA_ACCESSID empty; load .env failed?")
		os.Exit(1)
	}

	connector.InitWithOptions(
		env.WithApiHost(httplib.URL_EU),
		env.WithMsgHost(httplib.MSG_EU),
		env.WithAppName("tuyaSDK"),
		env.WithDebugMode(true),
	)

	now := time.Now()
	effective := hourUnix(now)
	invalid := hourUnix(now.Add(2 * time.Hour))

	fmt.Println("=== STEP 1: CREATE multiple offline password ===")
	fmt.Printf("effective=%d invalid=%d (%s .. %s local)\n",
		effective, invalid,
		time.Unix(effective, 0).Local(),
		time.Unix(invalid, 0).Local())

	createResp, err := postOffline(map[string]any{
		"name":           "spike-clear-test",
		"effective_time": effective,
		"invalid_time":   invalid,
		"type":           "multiple",
	})
	if err != nil {
		fmt.Println("CREATE transport error:", err)
		os.Exit(1)
	}
	createJSON, _ := json.MarshalIndent(createResp, "", "  ")
	fmt.Println(string(createJSON))

	if !createResp.Success || createResp.Result.OfflineTempPasswordID == "" {
		fmt.Println("CREATE failed or empty password_id — cannot test clear_one")
		os.Exit(1)
	}

	pwdID := createResp.Result.OfflineTempPasswordID
	fmt.Println("=== STEP 2: CLEAR_ONE password_id=", pwdID, "===")

	clearResp, err := postOffline(map[string]any{
		"name":         "spike-clear-test",
		"type":         "clear_one",
		"password_id":  pwdID,
		// docs: CLEAR_ONE may still want hour-aligned times
		"effective_time": effective,
		"invalid_time":   invalid,
	})
	if err != nil {
		fmt.Println("CLEAR_ONE transport error:", err)
		os.Exit(1)
	}
	clearJSON, _ := json.MarshalIndent(clearResp, "", "  ")
	fmt.Println(string(clearJSON))

	fmt.Println("=== SUMMARY ===")
	fmt.Printf("create success=%v id=%s pin_len=%d\n",
		createResp.Success, pwdID, len(createResp.Result.OfflineTempPassword))
	fmt.Printf("clear_one success=%v code=%d msg=%q pin=%q id=%q\n",
		clearResp.Success, clearResp.Code, clearResp.Msg,
		clearResp.Result.OfflineTempPassword, clearResp.Result.OfflineTempPasswordID)
}
