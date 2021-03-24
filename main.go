// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/percona/mongodb_exporter/config"
	"github.com/percona/mongodb_exporter/exporter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

//nolint:gochecknoglobals
var (
	version   string
	commit    string
	buildDate string
)

// GlobalFlags has command line flags to configure the exporter.
type GlobalFlags struct {
	CollStatsCollections    string `name:"mongodb.collstats-colls" help:"List of comma separared databases.collections to get $collStats" placeholder:"db1.col1,db2.col2"`
	IndexStatsCollections   string `name:"mongodb.indexstats-colls" help:"List of comma separared databases.collections to get $indexStats" placeholder:"db1.col1,db2.col2"`
	URI                     string `name:"mongodb.uri" help:"MongoDB connection URI" env:"MONGODB_URI" placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	GlobalConnPool          bool   `name:"mongodb.global-conn-pool" help:"Use global connection pool instead of creating new pool for each http request."`
	WebListenAddress        string `name:"web.listen-address" help:"Address to listen on for web interface and telemetry" default:":9216"`
	WebTelemetryPath        string `name:"web.telemetry-path" help:"Metrics expose path" default:"/metrics"`
	LogLevel                string `name:"log.level" help:"Only log messages with the given severuty or above. Valid levels: [debug, info, warn, error, fatal]" enum:"debug,info,warn,error,fatal" default:"error"`
	DisableDiagnosticData   bool   `name:"disable.diagnosticdata" help:"Disable collecting metrics from getDiagnosticData"`
	DisableReplicasetStatus bool   `name:"disable.replicasetstatus" help:"Disable collecting metrics from replSetGetStatus"`
	CompatibleMode          bool   `name:"compatible-mode" help:"Enable old mongodb-exporter compatible metrics"`
	Version                 bool   `name:"version" help:"Show version and exit"`
}

func loadConfig() (*config.Config, error) {
	path, _ := os.Getwd()
	path = filepath.Join(path, "conf/conf.yml")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.New("read conf.yml fail")
	}
	conf := new(config.Config)
	err = yaml.Unmarshal(data, conf)
	if err != nil {
		return nil, errors.New("unmarshal conf.yml fail")
	}
	return conf, nil
}

func main() {
	var opts GlobalFlags
	_ = kong.Parse(&opts,
		kong.Name("mongodb_exporter"),
		kong.Description("MongoDB Prometheus exporter"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version,
		})

	if opts.Version {
		fmt.Println("mongodb_exporter - MongoDB Prometheus exporter")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build date: %s\n", buildDate)

		return
	}
	conf, err := loadConfig()
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	if opts.URI == "" {
		opts.URI = conf.DefaultTarget.Host + ":" + conf.DefaultTarget.Port
	}
	e, err := buildExporter(opts, conf)
	if err != nil {
		log.Fatal(err)
	}

	e.Run()
}

func buildExporter(opts GlobalFlags, config *config.Config) (*exporter.Exporter, error) {
	log := logrus.New()

	levels := map[string]logrus.Level{
		"debug": logrus.DebugLevel,
		"error": logrus.ErrorLevel,
		"fatal": logrus.FatalLevel,
		"info":  logrus.InfoLevel,
		"warn":  logrus.WarnLevel,
	}
	log.SetLevel(levels[opts.LogLevel])

	log.Debugf("Compatible mode: %v", opts.CompatibleMode)

	if !strings.HasPrefix(opts.URI, "mongodb") {
		log.Debugf("Prepending mongodb:// to the URI")
		opts.URI = "mongodb://" + opts.URI
	}

	log.Debugf("Connection URI: %s", opts.URI)

	exporterOpts := &exporter.Opts{
		CollStatsCollections:    strings.Split(opts.CollStatsCollections, ","),
		CompatibleMode:          opts.CompatibleMode,
		IndexStatsCollections:   strings.Split(opts.IndexStatsCollections, ","),
		Logger:                  log,
		Path:                    opts.WebTelemetryPath,
		URI:                     opts.URI,
		GlobalConnPool:          opts.GlobalConnPool,
		WebListenAddress:        opts.WebListenAddress,
		DisableDiagnosticData:   opts.DisableDiagnosticData,
		DisableReplicasetStatus: opts.DisableReplicasetStatus,
		Config:                  config,
	}

	e, err := exporter.New(exporterOpts)
	if err != nil {
		return nil, err
	}

	return e, nil
}
