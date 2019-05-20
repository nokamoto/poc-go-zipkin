package main

import (
	"flag"
	"fmt"
	pb "github.com/nokamoto/poc-go-zipkin/service"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"net"
	"time"
)

var port = flag.Int("port", 9090, "grpc server port")
var addr = flag.String("addr", "localhost:9090", "grpc client dial addr")

func serve() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(fmt.Sprintf("listen tcp port (%d) - %v", *port, err))
	}

	fmt.Printf("listen tcp port (%d)\n", *port)

	opts := []grpc.ServerOption{}
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

	conn, err := grpc.Dial(*addr, opts...)
	if err != nil {
		panic(fmt.Sprintf("err: %s %v", *addr, err))
	}
	defer conn.Close()

	client := pb.NewServiceAClient(conn)

	for {
		fmt.Println("call")
		ctx := context.Background()
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(3000)*time.Millisecond))

		res, err := client.Send(ctx, &pb.Request{})
		if err != nil {
			fmt.Printf("err: %v\n", err)

			if grpc.Code(err) == codes.DeadlineExceeded {
				cancel()
			}
		} else {
			fmt.Printf("rec: %v\n", res)
		}

		time.Sleep(time.Duration(5000) * time.Millisecond)
	}
}

func main() {
	flag.Parse()

	go call()

	serve()
}
