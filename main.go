package main

import (
	"flag"
	"github.com/globalsign/mgo"
	_ "github.com/micro/go-plugins/broker/rabbitmq"
	_ "github.com/micro/go-plugins/registry/kubernetes"
	_ "github.com/micro/go-plugins/transport/grpc"
	"github.com/paysuper/paysuper-management-api/api"
	"github.com/paysuper/paysuper-management-api/config"
	"github.com/paysuper/paysuper-management-api/database"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// @title Protocol One payment solution swagger documentation
// @version 1.0
// @description This is a Protocol One payment solution service.
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host p1payapi.tst.protocol.one
func main() {
	migration := flag.String("migration", "", "run database migrations with specified direction")
	flag.Parse()

	err, conf := config.NewConfig()

	if err != nil {
		log.Fatalln(err)
	}

	db, err := database.NewConnection(&conf.Database)

	if err != nil {
		log.Fatalf("database connection failed with error: %s\n", err)
	}

	defer db.Close()

	if *migration != "" {
		err := database.Migrate(db.Database().(*mgo.Database), *migration)

		if err != nil {
			log.Fatalf("database migration failed with error: %s\n", err)
		}

		return
	}

	logger, err := zap.NewProduction()

	if err != nil {
		log.Fatalf("Application logger initialization failed with error: %s\n", err)
	}
	zap.ReplaceGlobals(logger)

	defer func() {
		if err := logger.Sync(); err != nil {
			return
		}
	}()

	sugar := logger.Sugar()

	defer func() {
		if err := sugar.Sync(); err != nil {
			return
		}
	}()

	sInit := &api.ServerInitParams{
		Config:      conf,
		Database:    db,
		Logger:      sugar,
		HttpScheme:  conf.HttpScheme,
		K8sHost:     conf.KubernetesHost,
		AmqpAddress: conf.AmqpAddress,
		Auth1:       &conf.Auth1,
	}

	server, err := api.NewServer(sInit)

	if err != nil {
		log.Fatalf("server crashed on init with error: %s\n", err)
	}

	err = server.Start()

	if err != nil {
		log.Fatalf("server crashed on start with error: %s\n", err)
	}

	handleOsSignals(server)
}

func handleOsSignals(server *api.Api) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	exitChan := make(chan int)

	go func() {
		for {
			s := <-signalChan
			switch s {
			case os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT:
				server.Stop()
				exitChan <- 0
			}
		}
	}()

	code := <-exitChan
	os.Exit(code)
}
