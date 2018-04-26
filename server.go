package main

import (
	"fmt"
	//"net"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/term"
	"github.com/go-kit/kit/metrics"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	//"github.com/newtonsystems/grpc_types/go/grpc_types"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	//"golang.org/x/net/context"
	//"google.golang.org/grpc"
	//"google.golang.org/grpc/reflection"
	"github.com/streadway/amqp"
)

const (
	grpcPort = ":50000"
	httpPort = ":9090"
)

type Server struct{}

type instrumentingMiddleware struct {
	pings metrics.Counter
}

var logger = GetLogger()

var ints = CreateMetrics()

func CreateMetrics() metrics.Counter {
	var ints metrics.Counter
	{
		// Business-level metrics.
		ints = kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "monitoring",
			Subsystem: "ping",
			Name:      "integers_summed",
			Help:      "Total count of integers summed via the Sum method.",
		}, []string{})

	}

	return ints
}

// GetLogger get logger
func GetLogger() log.Logger {
	// Color by level value
	colorFn := func(keyvals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyvals)-1; i += 2 {
			if keyvals[i] != "level" {
				continue
			}
			switch keyvals[i+1] {
			case "debug":
				return term.FgBgColor{Fg: term.DarkGray}
			case "info":
				return term.FgBgColor{Fg: term.DarkGreen}
			case "warn":
				return term.FgBgColor{Fg: term.Yellow, Bg: term.White}
			case "error":
				return term.FgBgColor{Fg: term.Red}
			case "crit":
				return term.FgBgColor{Fg: term.Gray, Bg: term.DarkRed}
			default:
				return term.FgBgColor{}
			}
		}
		return term.FgBgColor{}
	}

	var logger log.Logger
	{
		logger = term.NewLogger(os.Stdout, log.NewLogfmtLogger, colorFn)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
		logger = log.With(logger, "service", "ping")
	}

	return logger
}

// // Ping responses with a message back and
// func (s *Server) Ping(ctx context.Context, in *grpc_types.PingRequest) (*grpc_types.PingResponse, error) {
// 	logger.Log("level", "info", "method", "Ping", "msg", "Received ping from service: "+in.Message, "ints", fmt.Sprintf("%v", ints))
// 	ints.Add(1)
// 	return &grpc_types.PingResponse{Message: "Hello " + in.Message}, nil
// }

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {
	started := time.Now()

	logger.Log("level", "info", "msg", "starting ...")
	defer logger.Log("msg", "goodbye")

	// --- Probes ----------------------------------------------------------

	// Liveness probe
	http.HandleFunc("/started", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		data := (time.Now().Sub(started)).String()
		w.Write([]byte(data))
	})

	// Readiness probe
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		duration := time.Now().Sub(started)
		if duration.Seconds() > 10 {
			w.WriteHeader(200)
			w.Write([]byte("ok"))
		} else {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("error: %v", duration.Seconds())))
		}
	})

	// Readiness probe
	http.HandleFunc("/rabbit", func(w http.ResponseWriter, r *http.Request) {
		conn, err := amqp.Dial("amqp://guest:guest@rabbitmqa.ft.kubecl.geneity:15672")
		failOnError(err, "Failed to connect to RabbitMQ")
		defer conn.Close()

		ch, err := conn.Channel()
		failOnError(err, "Failed to open a channel")
		defer ch.Close()

		q, err := ch.QueueDeclare(
			"hello", // name
			false,   // durable
			false,   // delete when unused
			false,   // exclusive
			false,   // no-wait
			nil,     // arguments
		)
		failOnError(err, "Failed to declare a queue")

		msgs, err := ch.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto-ack
			false,  // exclusive
			false,  // no-local
			false,  // no-wait
			nil,    // args
		)
		failOnError(err, "Failed to register a consumer")

		forever := make(chan bool)

		go func() {
			for d := range msgs {
				log.Printf("Received a message: %s", d.Body)
			}
		}()

		log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
		<-forever

	})

	http.DefaultServeMux.Handle("/metrics", promhttp.Handler())

	logger.Log("level", "debug", "transport", "http", "port", httpPort, "msg", "running http probe server ...")
	http.ListenAndServe(httpPort, nil)

	// --- End of Probes ---------------------------------------------------

	// // Listen on port
	// lis, err := net.Listen("tcp", grpcPort)
	// if err != nil {
	// 	logger.Log("level", "error", "msg", "Failed to listen on port %s: %v", grpcPort, err)
	// 	return
	// }
	// defer lis.Close()

	// // Setup gRPC
	// s := grpc.NewServer()
	// grpc_types.RegisterPingServer(s, &Server{})
	// defer s.GracefulStop()

	// // Register reflection service on gRPC server.
	// reflection.Register(s)

	// Serve
	// logger.Log("level", "debug", "transport", "http", "port", grpcPort, "msg", "running grpc server ...")
	// if err := s.Serve(lis); err != nil {
	// 	logger.Log("level", "error", "msg", "Failed to serve on port %s : %v", grpcPort, err)
	// 	return
	// }

}
