// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/percona/exporter_shared"
	"github.com/percona/mongodb_exporter/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"

	pmmVersion "github.com/percona/pmm/version"

	"github.com/percona/mongodb_exporter/collector"
	"github.com/percona/mongodb_exporter/shared"
)

const (
	program           = "mongodb_exporter"
	versionDataFormat = "20060102-15:04:05"
)

var (
	listenAddressF = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9216").String()
	metricsPathF   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	collectDatabaseF             = kingpin.Flag("collect.database", "Enable collection of Database metrics").Bool()
	collectCollectionF           = kingpin.Flag("collect.collection", "Enable collection of Collection metrics").Bool()
	collectTopF                  = kingpin.Flag("collect.topmetrics", "Enable collection of table top metrics").Bool()
	collectIndexUsageF           = kingpin.Flag("collect.indexusage", "Enable collection of per index usage stats").Bool()
	mongodbCollectConnPoolStatsF = kingpin.Flag("collect.connpoolstats", "Collect MongoDB connpoolstats").Bool()
	uriF                         = kingpin.Flag("mongodb.uri", "MongoDB URI, format").
					PlaceHolder("[mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]").
					Default("mongodb://localhost:27017").
					Envar("MONGODB_URI").
					String()
	testF = kingpin.Flag("test", "Check MongoDB connection, print buildInfo() information and exit.").Bool()
)

func loadConfig() (*config.Config, error) {
	path, _ := os.Getwd()
	path = filepath.Join(path, "conf/conf.yml")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.New("read conf.yml fail:" + path)
	}
	conf := new(config.Config)
	err = yaml.Unmarshal(data, conf)
	if err != nil {
		return nil, errors.New("unmarshal conf.yml fail")
	}
	return conf, nil
}

func main() {
	initVersionInfo()
	log.AddFlags(kingpin.CommandLine)
	kingpin.Parse()

	if *testF {
		buildInfo, err := shared.TestConnection(
			shared.MongoSessionOpts{
				URI: *uriF,
			},
		)
		if err != nil {
			log.Errorf("Can't connect to MongoDB: %s", err)
			os.Exit(1)
		}
		fmt.Println(string(buildInfo))
		os.Exit(0)
	}

	// TODO: Maybe we should move version.Info() and version.BuildContext() to https://github.com/percona/exporter_shared
	// See: https://jira.percona.com/browse/PMM-3250 and https://github.com/percona/mongodb_exporter/pull/132#discussion_r262227248
	log.Infoln("Starting", program, version.Info())
	log.Infoln("Build context", version.BuildContext())

	programCollector := version.NewCollector(program)

	conf, _ := loadConfig()

	handler := handler(programCollector, conf.MongoModules)
	//exporter_shared.RunServer("MongoDB", e.webListenAddress, e.path, handler)
	//promHandler := promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
	exporter_shared.RunServer("MongoDB", *listenAddressF, *metricsPathF, handler)
}

func getTarget(module string, target string, mongoList []*config.MongoModule) (string, error) {
	arr := strings.Split(target, ":")
	if len(arr) < 2 {
		return "", errors.New("target error")
	}
	if module == "" {
		return "", errors.New("uri error, not found module")
	}
	ip := arr[0]
	port := arr[1]
	var targetValue string

	var user string
	var password string
	for i := 0; i < len(mongoList); i++ {
		if mongoList[i].Name == module {
			user = mongoList[i].User
			password = mongoList[i].Password
			break
		}
	}
	if user == "" || password == "" {
		return "", errors.New("not found module in conf.yml")
	}
	targetValue = user + ":" + password + "@" + ip + ":" + port

	return targetValue, nil
}

func handler(programCollector prometheus.Collector, modules []*config.MongoModule) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri := r.URL.Query()
		target := uri.Get("target")
		module := uri.Get("module")
		targetUri := *uriF
		if target != "" {
			targetValue, err := getTarget(module, target, modules)
			if err != nil {
				buf := "get exporter target fail: " + err.Error()
				w.Write([]byte(buf))
				return
			}
			if !strings.HasPrefix(targetValue, "mongodb://") {
				targetValue = "mongodb://" + targetValue
			}
			targetUri = targetValue
		}
		fmt.Println(targetUri)
		mongodbCollector := collector.NewMongodbCollector(&collector.MongodbCollectorOpts{
			URI:                      targetUri,
			CollectDatabaseMetrics:   *collectDatabaseF,
			CollectCollectionMetrics: *collectCollectionF,
			CollectTopMetrics:        *collectTopF,
			CollectIndexUsageStats:   *collectIndexUsageF,
			CollectConnPoolStats:     *mongodbCollectConnPoolStatsF,
		})

		//prometheus.MustRegister(programCollector, mongodbCollector)

		registry := prometheus.NewRegistry()
		registry.MustRegister(mongodbCollector)

		gatherers := prometheus.Gatherers{}
		gatherers = append(gatherers, prometheus.DefaultGatherer)
		gatherers = append(gatherers, registry)

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		})

		h.ServeHTTP(w, r)
	})
}

// initVersionInfo sets version info
// If binary was build for PMM with environment variable PMM_RELEASE_VERSION
// `--version` will be displayed in PMM format. Also `PMM Version` will be connected
// to application version and will be printed in all logs.
// TODO: Refactor after moving version.Info() and version.BuildContext() to https://github.com/percona/exporter_shared
// See: https://jira.percona.com/browse/PMM-3250 and https://github.com/percona/mongodb_exporter/pull/132#discussion_r262227248
func initVersionInfo() {
	version.Version = pmmVersion.Version
	version.Revision = pmmVersion.FullCommit
	version.Branch = pmmVersion.Branch

	if buildDate, err := strconv.ParseInt(pmmVersion.Timestamp, 10, 64); err != nil {
		version.BuildDate = time.Unix(0, 0).Format(versionDataFormat)
	} else {
		version.BuildDate = time.Unix(buildDate, 0).Format(versionDataFormat)
	}

	if pmmVersion.PMMVersion != "" {
		version.Version += "-pmm-" + pmmVersion.PMMVersion
		kingpin.Version(pmmVersion.FullInfo())
	} else {
		kingpin.Version(version.Print(program))
	}

	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.Help = fmt.Sprintf("%s exports various MongoDB metrics in Prometheus format.\n", pmmVersion.ShortInfo())
}
