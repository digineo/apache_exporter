package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

var (
	// CLI flags
	listeningAddress = ":9117"
	metricsEndpoint  = "/metrics"
	insecure         = false
	showVersion      = false

	defaultTarget = "http://localhost/server-status?auto"
	client        *http.Client
)

func main() {
	flag.StringVar(&listeningAddress, "telemetry.address", listeningAddress, "Address on which to expose metrics")
	flag.BoolVar(&insecure, "insecure", insecure, "Ignore server certificate if using https")
	flag.BoolVar(&showVersion, "version", showVersion, "Print version information")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Print("apache_exporter"))
		os.Exit(0)
	}

	client = &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		},
	}

	log.Infoln("Starting apache_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting Server: %s", listeningAddress)

	http.HandleFunc(metricsEndpoint, func(w http.ResponseWriter, r *http.Request) {
		reg := prometheus.NewRegistry()

		if r.FormValue("runtime") != "false" {
			reg.MustRegister(version.NewCollector("apache_exporter"))
			reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
			reg.MustRegister(prometheus.NewGoCollector())
		}

		if target := r.FormValue("target"); target != "false" {
			if target == "" {
				target = defaultTarget
			}
			reg.MustRegister(NewExporter(target))
		}

		h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, landingPage)
	})
	log.Fatal(http.ListenAndServe(listeningAddress, nil))
}

const landingPage = `<!doctype html><html>
<head>
	<meta charset="UTF-8">
	<title>Apache Exporter</title>
</head>
<body>
	<h1>Apache Exporter</h1>
	<ul>
		<li><a href="/metrics">Runtime Metrics with default probe</a></li>
		<li><a href="/metrics?runtime=false&target=http%3A%2F%2Flocalhost%2Fserver-status%3Fauto>Runtime Metrics with specific probe</a></li>
		<li><a href="/metrics?target=false">Runtime Metrics without probe</a></li>
		<li><a href="/metrics?runtime=false&target=http%3A%2F%2Flocalhost%2Fserver-status%3Fauto">Only http://localhost/server-status?auto</a></li>
	</ul>
	<p><a href="https://github.com/digineo/apache_exporter">Sources</a></p>
</body>
</html>
`
