package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tas1999/tuya-connector-go/connector"
	"github.com/tas1999/tuya-connector-go/connector/logger"
	// "main/gen/go/password"
)

// protoc -I tuya-smart-lock-proto tuya-smart-lock-proto/password.proto --go_out=./gen/go/ --go_opt=paths=source_relative --go-grpc_out=./gen/go/ --go-grpc_opt=paths=source_relative
type GenerateOfflineTemporaryPasswordReq struct {
	Name          string       `json:"name"`
	EffectiveTime int64        `json:"effective_time"`
	InvalidTime   int64        `json:"invalid_time"`
	PasswordType  PasswordType `json:"type"`
}

type PasswordType string

const MULTIPLE PasswordType = "multiple"

type GenerateOfflineTemporaryPasswordResponse struct {
	Success bool  `json:"success"`
	T       int64 `json:"t"`
	Result  struct {
		EffectiveTime           int    `json:"effective_time"`
		InvalidTime             int    `json:"invalid_time"`
		OfflineTempPassword     string `json:"offline_temp_password"`
		OfflineTempPasswordID   string `json:"offline_temp_password_id"`
		OfflineTempPasswordName string `json:"offline_temp_password_name"`
	} `json:"result"`

	// error info
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// type PasswordService struct{

// }
// func(p *PasswordService) GenerateOfflineTemporaryPassword(context.Context, *GenerateOfflineTemporaryPasswordRequest) (*GenerateOfflineTemporaryPasswordResponse, error){

// }

func GenerateNewPassword(req GenerateOfflineTemporaryPasswordReq) (string, error) {

	device_id := "bff1264dbab03af880ru11"
	resp := &GenerateOfflineTemporaryPasswordResponse{}
	body, err := json.Marshal(req)

	if err != nil {
		return "", err
	}

	err = connector.MakePostRequest(
		context.Background(),
		connector.WithAPIUri(fmt.Sprintf("/v1.1/devices/%s/door-lock/offline-temp-password", device_id)),
		connector.WithPayload(body),
		connector.WithResp(resp))
	if err != nil {
		logger.Log.Errorf("err:%s", err.Error())
		return "", err
	}
	logger.Log.Debug(resp)
	return resp.Result.OfflineTempPassword, nil
}
