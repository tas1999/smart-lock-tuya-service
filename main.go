package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/tas1999/tuya-connector-go/connector"
	"github.com/tas1999/tuya-connector-go/connector/constant"
	"github.com/tas1999/tuya-connector-go/connector/env"
	"github.com/tas1999/tuya-connector-go/connector/env/extension"
	"github.com/tas1999/tuya-connector-go/connector/httplib"
	"github.com/tas1999/tuya-connector-go/connector/logger"
	"github.com/tas1999/tuya-connector-go/connector/message/event"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		logger.Log.Fatal("Error loading .env file")
	}
}

func main() {
	connector.InitWithOptions(env.WithApiHost(httplib.URL_EU),
		env.WithMsgHost(httplib.MSG_EU),
		env.WithAppName("tuyaSDK"),
		env.WithDebugMode(true))

	go Listener()
	watitSignal()
}
func watitSignal() {
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	c := <-quitCh
	extension.GetMessage(constant.TUYA_MESSAGE).Stop()
	logger.Log.Infof("receive sig:%v, shutdown the http server...", c.String())
}
func Listener() {
	r := extension.GetMessage(constant.TUYA_MESSAGE)
	r.InitMessageClient()
	extension.GetMessage(constant.TUYA_MESSAGE).SubEventMessage(func(m *event.DevicePropertyMessage) {
		logger.Log.Info("=========== DevicePropertyMessage： ==========")
		for _, v := range m.BizData.Properties {
			logger.Log.Info(v.Code, v.Value)
		}
	})
}
