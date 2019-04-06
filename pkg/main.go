package main

import (
	"context"
	"net"
	"strconv"

	"github.com/spf13/cobra"

	healthv1 "github.com/kraman/grpc-ms/pkg/api/grpc/health/v1"
	samplev1 "github.com/kraman/grpc-ms/pkg/api/kraman/sample/v1"
	"github.com/kraman/grpc-ms/pkg/server"
)

var rootCmd = &cobra.Command{
	Use: "service",
	Run: func(cmd *cobra.Command, args []string) {
		serviceName, _ := cmd.Flags().GetString("name")
		podIP, _ := cmd.Flags().GetIP("pod-ip")
		numBuckets, _ := cmd.Flags().GetInt("num-buckets")
		port, _ := cmd.Flags().GetInt("port")

		s := server.New(serviceName, podIP.String(), numBuckets)
		healthService := healthv1.New()
		sampleService := samplev1.New()

		s.RegisterService(healthService)
		s.RegisterService(sampleService)
		healthService.RegisterService(healthv1.HealthReporter(sampleService))

		s.ListenAndServe(context.Background(), strconv.Itoa(port))
	},
}

func init() {
	rootCmd.Flags().String("name", "grpc-ms", "Service name")
	rootCmd.Flags().Int("port", 8177, "Service port")
	rootCmd.Flags().IP("pod-ip", net.ParseIP("127.0.0.1"), "Pod IP")
	rootCmd.Flags().Int("num-buckets", 10, "Number of buckets")
}

func main() {
	rootCmd.Execute()
}
