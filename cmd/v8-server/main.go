package main

import (
	"context"
	"os"
	"os/signal"
  "database/sql"
	grpcserver "github.com/contiamo/goserver/grpc"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
  _ "github.com/lib/pq"

  "github.com/trusch/v8-server/pkg/api"
  "github.com/trusch/v8-server/pkg/server"

)

var (
	dbStr     = pflag.String("db", "postgres://postgres@localhost:5432?sslmode=disable", "postgres connect string")
	listenAddr = pflag.String("listen", ":3001", "listening address")
)

func main() {
	pflag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)


	db, err := sql.Open("postgres", *dbStr)
	if err != nil {
		logrus.Fatalf("failed to open db: %v", err)
	}

	// setup grpc server with options
	grpcServer, err := grpcserver.New(&grpcserver.Config{
		Options: []grpcserver.Option{
			grpcserver.WithCredentials("", "", ""),
			grpcserver.WithLogging("v8-server"),
			grpcserver.WithMetrics(),
			grpcserver.WithRecovery(),
			grpcserver.WithReflection(),
		},
		Extras: []grpc.ServerOption{
			grpc.MaxSendMsgSize(1 << 12),
		},
		Register: func(srv *grpc.Server) {
      api.RegisterV8Server(srv, server.New(db))
		},
	})
	if err != nil {
		logrus.Fatal(err)
	}

	// start server
	go func() {
		if err := grpcserver.ListenAndServe(ctx, *listenAddr, grpcServer); err != nil {
			logrus.Fatal(err)
		}
	}()

	<-c
	cancel()
}
