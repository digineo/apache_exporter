# Apache Exporter for Prometheus

Exports Apache mod_status statistics via HTTP for Prometheus consumption.

With working golang environment it can be built with `go get`.  There is
a [good article][machineperson] with build HOWTO and usage example.

# Usage

```console
$ apache_exporter -help
Usage of apache_exporter:
  -insecure
    	Ignore server certificate if using https
  -telemetry.address string
    	Address on which to expose metrics (default ":9117")
  -version
    	Print version information
```

Tested on Apache 2.2 and Apache 2.4.

If your server-status page is secured by http auth, add the credentials
to the scrape URL following this example:

```
http://user:password@localhost/server-status?auto
```

# Using Docker

## Build the compatible binary

To make sure that exporter binary created by build job is suitable to run
on busybox environment, generate the binary using Makefile definition.
Inside project directory run:

```console
$ make
```

*Please be aware that binary generated using `go get` or `go build` with defaults may not work in busybox/alpine base images.*

## Build image

Run the following commands from the project root directory.

```console
$ docker build -t apache_exporter .
```

## Run

```console
$ docker run -d -p 9117:9117 apache_exporter
```

## Collectors

Apache metrics:

```
# HELP apache_accesses_total Current total apache accesses (*)
# TYPE apache_accesses_total counter
# HELP apache_scoreboard Apache scoreboard statuses
# TYPE apache_scoreboard gauge
# HELP apache_sent_bytes_total Current total kbytes sent (*)
# TYPE apache_sent_bytes_total counter
# HELP apache_cpu_load CPU Load (*)
# TYPE apache_cpu_load gauge
# HELP apache_up Could the apache server be reached
# TYPE apache_up gauge
# HELP apache_uptime_seconds_total Current uptime in seconds (*)
# TYPE apache_uptime_seconds_total counter
# HELP apache_workers Apache worker statuses
# TYPE apache_workers gauge
```

Exporter process metrics:

```
# HELP http_request_duration_microseconds The HTTP request latencies in microseconds.
# TYPE http_request_duration_microseconds summary
# HELP http_request_size_bytes The HTTP request sizes in bytes.
# TYPE http_request_size_bytes summary
# HELP http_response_size_bytes The HTTP response sizes in bytes.
# TYPE http_response_size_bytes summary
# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds.
# TYPE process_cpu_seconds_total counter
# HELP process_max_fds Maximum number of open file descriptors.
# TYPE process_max_fds gauge
# HELP process_open_fds Number of open file descriptors.
# TYPE process_open_fds gauge
# HELP process_resident_memory_bytes Resident memory size in bytes.
# TYPE process_resident_memory_bytes gauge
# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
# HELP process_virtual_memory_bytes Virtual memory size in bytes.
# TYPE process_virtual_memory_bytes gauge
```

Metrics marked '(*)' are only available if `ExtendedStatus` is `On` in
Apache webserver configuration. In version 2.3.6, loading `mod_status`
will toggle `ExtendedStatus On` by default.

## FAQ

Q. Can you add additional metrics such as reqpersec, bytespersec and bytesperreq?

A. In line with the [best practices][], the exporter only provides the
totals and you should derive rates using [PromQL][].


## Authors

The exporter was originally created by [neezgee](https://github.com/neezgee)
and maintained by [Lusitaniae](https://github.com/Lusitaniae).


[machineperson]: https://machineperson.github.io/monitoring/2016/01/04/exporting-apache-metrics-to-prometheus.html
[best practices]: https://prometheus.io/docs/instrumenting/writing_exporters/#drop-less-useful-statistics
[PromQL]: https://prometheus.io/docs/prometheus/latest/querying/basics/
