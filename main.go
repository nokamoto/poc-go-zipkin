package main

import (
	"contrib.go.opencensus.io/exporter/zipkin"
	"flag"
	"fmt"
	pb "github.com/nokamoto/poc-go-zipkin/service"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"time"
)

var port = flag.Int("port", 9090, "grpc server port")
var addr = flag.String("addr", "localhost:9090", "grpc client dial addr")
var reporterURI = flag.String("reporter", "http://zipkin:9411/api/v2/spans", "zipkin reporter endpoint")

func serve() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(fmt.Sprintf("listen tcp port (%d) - %v", *port, err))
	}

	fmt.Printf("listen tcp port (%d)\n", *port)

	opts := []grpc.ServerOption{}
	opts = append(opts, grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	server := grpc.NewServer(opts...)

	a, err := newServiceA()
	if err != nil {
		panic(err)
	}

	pb.RegisterServiceAServer(server, a)
	pb.RegisterServiceBServer(server, &serviceB{})
	reflection.Register(server)

	fmt.Println("ready to serve")
	err = server.Serve(lis)
	if err != nil {
		panic(fmt.Sprintf("serve %v - %v", lis, err))
	}
}

func call() {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithStatsHandler(new(ocgrpc.ClientHandler)))

	conn, err := grpc.Dial(*addr, opts...)
	if err != nil {
		panic(fmt.Sprintf("err: %s %v", *addr, err))
	}
	defer conn.Close()

	client := pb.NewServiceAClient(conn)

	for {
		fmt.Println("call")
		ctx, span := trace.StartSpan(context.Background(), "ClientCall")
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(3000)*time.Millisecond))

		res, err := client.Send(ctx, &pb.Request{})
		if err != nil {
			span.SetStatus(trace.Status{Code: int32(grpc.Code(err)), Message: grpc.ErrorDesc(err)})
			fmt.Printf("err: %v\n", err)

			if grpc.Code(err) == codes.DeadlineExceeded {
				cancel()
			}
		} else {
			fmt.Printf("rec: %v\n", res)
		}
		span.End()

		time.Sleep(time.Duration(5000) * time.Millisecond)
	}
}

func export() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	localEndpointURI := fmt.Sprintf("%s:%d", host, *port)
	serviceName := "poc-go-zipkin"

	localEndpoint, err := openzipkin.NewEndpoint(serviceName, localEndpointURI)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Zipkin localEndpoint with URI %s error: %v", localEndpointURI, err))
	}

	reporter := zipkinHTTP.NewReporter(*reporterURI)
	ze := zipkin.NewExporter(reporter, localEndpoint)

	trace.RegisterExporter(ze)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
}

func main() {
	flag.Parse()

	go call()

	export()

	serve()
}
