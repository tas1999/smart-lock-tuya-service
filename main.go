package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/tas1999/smart-lock-tuya-service/internal/auth"
	"github.com/tas1999/smart-lock-tuya-service/internal/config"
	"github.com/tas1999/smart-lock-tuya-service/internal/events"
	pwd "github.com/tas1999/smart-lock-tuya-service/internal/password"
	"github.com/tas1999/smart-lock-tuya-service/internal/webhook"
	pb "github.com/tas1999/smart-lock-tuya-service/gen/go"
	"github.com/tas1999/tuya-connector-go/connector"
	"github.com/tas1999/tuya-connector-go/connector/constant"
	"github.com/tas1999/tuya-connector-go/connector/env"
	"github.com/tas1999/tuya-connector-go/connector/env/extension"
	"github.com/tas1999/tuya-connector-go/connector/httplib"
	"github.com/tas1999/tuya-connector-go/connector/logger"
	"google.golang.org/grpc"
)

func init() {
	_ = godotenv.Load()
}

func main() {
	cfg := config.FromEnv()
	if cfg.TuyaAccessID == "" || cfg.TuyaAccessKey == "" {
		logger.Log.Fatal("TUYA_ACCESSID and TUYA_ACCESSKEY are required")
	}
	if cfg.APIKey == "" {
		logger.Log.Fatal("API_KEY is required")
	}

	connector.InitWithOptions(
		env.WithApiHost(httplib.URL_EU),
		env.WithMsgHost(httplib.MSG_EU),
		env.WithAppName("tuyaSDK"),
		env.WithDebugMode(os.Getenv("TUYA_DEBUG") == "true"),
	)

	wh := &webhook.Client{URL: cfg.WebhookURL, APIKey: cfg.APIKey}
	listener := &events.Listener{Cfg: cfg, Webhook: wh}
	go listener.Start()
	logger.Log.Infof("event filter: %s", events.DeviceFilterSummary(cfg))

	go serveGRPC(cfg)

	waitSignal()
}

func serveGRPC(cfg config.Config) {
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		logger.Log.Fatalf("listen %s: %v", cfg.GRPCAddr, err)
	}
	srv := grpc.NewServer(grpc.UnaryInterceptor(auth.UnaryInterceptor(cfg.APIKey)))
	pb.RegisterPasswordServiceServer(srv, &pwd.GRPCServer{
		Client: &pwd.OnlineClient{AccessSecret: cfg.TuyaAccessKey},
	})
	logger.Log.Infof("gRPC listening on %s", cfg.GRPCAddr)
	if err := srv.Serve(lis); err != nil {
		logger.Log.Fatalf("gRPC serve: %v", err)
	}
}

func waitSignal() {
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	c := <-quitCh
	extension.GetMessage(constant.TUYA_MESSAGE).Stop()
	logger.Log.Infof("receive sig:%v, shutting down...", c.String())
}
