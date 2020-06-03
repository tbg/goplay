package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Create a summary to track fictional interservice RPC latencies for three
	// distinct services with different latency distributions. These services are
	// differentiated via a "service" label.
	rpcDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "rpc_durations_seconds",
			Help:       "RPC latency distributions.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"tenant"},
	)
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(rpcDurations)
}

func main() {
	flag.Parse()

	addr := "127.0.0.1:8080"

	go func() {
		// PS: there's also this which looks interesting generally: Looks
		// like we could attach, say, a SQL query or batch to a measured
		// latency.
		_ = prometheus.NewHistogram(prometheus.HistogramOpts{}).(prometheus.ExemplarObserver)

		for i := 0; i < 1000; i++ {
			// Chose random number between 0 and 1, map it to a tenant and ob-
			// serve it under that tenant. For example, tenant 9 should only
			// observe measurements from [0.9, 1).
			v := rand.Float64()                   // float64([0,1))
			tenantID := fmt.Sprint(int64(10 * v)) // 0, 1, ..., 9
			rpcDurations.WithLabelValues(tenantID).Observe(v)
			time.Sleep(10 * time.Millisecond)
			// And in fact we do:
			//
			// # HELP rpc_durations_seconds RPC latency distributions.
			// # TYPE rpc_durations_seconds summary
			// rpc_durations_seconds{tenant="0",quantile="0.5"} 0.05683202156480986
			// rpc_durations_seconds{tenant="0",quantile="0.9"} 0.09745461839911657
			// rpc_durations_seconds{tenant="0",quantile="0.99"} 0.09972966371993512
			// rpc_durations_seconds_sum{tenant="0"} 5.254874956640846
			// rpc_durations_seconds_count{tenant="0"} 96
			// rpc_durations_seconds{tenant="1",quantile="0.5"} 0.15651925473279124
			// rpc_durations_seconds{tenant="1",quantile="0.9"} 0.18724610140105305
			// rpc_durations_seconds{tenant="1",quantile="0.99"} 0.19856153537434532
			// rpc_durations_seconds_sum{tenant="1"} 10.079939830475233
			// rpc_durations_seconds_count{tenant="1"} 66
			//
			// [...]
			//
			// rpc_durations_seconds{tenant="8",quantile="0.5"} 0.8409932064362602
			// rpc_durations_seconds{tenant="8",quantile="0.9"} 0.8958032677441458
			// rpc_durations_seconds{tenant="8",quantile="0.99"} 0.8999048062725744
			// rpc_durations_seconds_sum{tenant="8"} 55.8734318857787
			// rpc_durations_seconds_count{tenant="8"} 66
			// rpc_durations_seconds{tenant="9",quantile="0.5"} 0.9508283197764046
			// rpc_durations_seconds{tenant="9",quantile="0.9"} 0.9854655786332479
			// rpc_durations_seconds{tenant="9",quantile="0.99"} 0.9991594902203332
			// rpc_durations_seconds_sum{tenant="9"} 71.20288738675224
			// rpc_durations_seconds_count{tenant="9"} 75
		}
		// We can remove data for tenants that we haven't seen in a long time.
		rpcDurations.Delete(prometheus.Labels{"tenant": "8"})
	}()

	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))
	log.Fatal(http.ListenAndServe(addr, nil))
}
