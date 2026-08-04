package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/tas1999/tuya-connector-go/connector"
	"github.com/tas1999/tuya-connector-go/connector/env"
	"github.com/tas1999/tuya-connector-go/connector/httplib"
)

func main() {
	_ = godotenv.Load()
	connector.InitWithOptions(
		env.WithApiHost(httplib.URL_EU),
		env.WithMsgHost(httplib.MSG_EU),
		env.WithAppName("tuyaSDK"),
		env.WithDebugMode(false),
	)

	var resp map[string]any
	err := connector.MakeGetRequest(
		context.Background(),
		connector.WithAPIUri("/v1.0/devices/bf22adc2ea2e5d66bdwdcc"),
		connector.WithResp(&resp),
	)
	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("err:", err)
	fmt.Println(string(b))
}
