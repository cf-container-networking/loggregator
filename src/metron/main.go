package main

import (
	"doppler/dopplerservice"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"

	"metron/clientpool"
	"metron/clientreader"
	"metron/networkreader"
	"metron/writers/batch"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventmarshaller"

	"logger"
	"metron/eventwriter"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/runtime_stats"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/pivotal-golang/localip"

	"metron/config"
	"signalmanager"
)

const (
	// This is 6061 to not conflict with any other jobs that might have pprof
	// running on 6060
	pprofPort = "6061"
	origin    = "MetronAgent"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("config", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

func main() {
	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

	flag.Parse()
	config, err := config.ParseConfig(*configFilePath)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	logger := logger.NewLogger(*debug, *logFilePath, "metron", config.Syslog)

	statsStopChan := make(chan struct{})
	batcher, _ := initializeMetrics(config, statsStopChan, logger)

	go func() {
		err := http.ListenAndServe(net.JoinHostPort("localhost", pprofPort), nil)
		if err != nil {
			logger.Errorf("Error starting pprof server: %s", err.Error())
		}
	}()

	logger.Info("Startup: Setting up the Metron agent")
	batchWriter, err := initializeDopplerPool(config, batcher, logger)
	if err != nil {
		panic(fmt.Errorf("Could not initialize doppler connection pool: %s", err))
	}

	//messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	//aggregator := messageaggregator.New(messageTagger, logger)
	//eventWriter.SetWriter(marshaller)

	//dropsondeUnmarshaller := eventunmarshaller.New(aggregator, batcher, logger)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", config.IncomingUDPPort)
	dropsondeReader, err := networkreader.New(metronAddress, "dropsondeAgentListener", batchWriter, logger)
	if err != nil {
		panic(fmt.Errorf("Failed to listen on %s: %s", metronAddress, err))
	}

	logger.Info("metron started")
	go dropsondeReader.Start()

	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()
	killChan := signalmanager.RegisterKillSignalChannel()

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			logger.Info("Shutting down")
			close(statsStopChan)
			return
		}
	}
}

func adapter(conf *config.Config, logger *gosteno.Logger) (storeadapter.StoreAdapter, error) {
	adapter, err := storeAdapterProvider(conf)
	if err != nil {
		return nil, err
	}
	err = adapter.Connect()
	if err != nil {
		logger.Warnd(map[string]interface{}{
			"error": err.Error(),
		}, "Failed to connect to etcd")
	}
	return adapter, nil
}

func initializeDopplerPool(conf *config.Config, batcher *metricbatcher.MetricBatcher, logger *gosteno.Logger) (eventmarshaller.BatchChainByteWriter, error) {
	adapter, err := adapter(conf, logger)
	if err != nil {
		return nil, err
	}
	clientPool := make(map[string]clientreader.ClientPool)
	writers := make(map[string]eventmarshaller.BatchChainByteWriter)

	ip, err := localip.LocalIP()
	if err != nil {
		return nil, err
	}

	proto := "tcp"
	tcpCreator := clientpool.NewTCPClientCreator(logger, nil)
	tcpWrapper := dopplerforwarder.NewWrapper(logger, proto)
	tcpPool := clientpool.NewDopplerPool(logger, tcpCreator)
	tcpForwarder := dopplerforwarder.New(tcpWrapper, tcpPool, logger)

	tcpBatchInterval := time.Duration(conf.TCPBatchIntervalMilliseconds) * time.Millisecond

	dropCounter := batch.NewDroppedCounter(tcpForwarder, batcher, origin, ip, conf)
	batchWriter, err := batch.NewWriter(
		"tcp",
		tcpForwarder,
		dropCounter,
		conf.TCPBatchSizeBytes,
		tcpBatchInterval,
		logger,
	)
	if err != nil {
		return nil, err
	}
	clientPool[proto] = tcpPool
	writers[proto] = batchWriter

	finder := dopplerservice.NewFinder(adapter, conf.LoggregatorDropsondePort, conf.Protocols.Strings(), conf.Zone, logger)
	finder.Start()

	//	marshaller := eventmarshaller.New(batcher, logger)

	go func() {
		for {
			protocol := clientreader.Read(clientPool, conf.Protocols.Strings(), finder.Next())
			logger.Infof("Chose protocol %s from last etcd event, updating writer...", protocol)
			//	marshaller.SetWriter(writers[protocol])
		}
	}()

	return batchWriter, nil
}

func initializeMetrics(config *config.Config, stopChan chan struct{}, logger *gosteno.Logger) (*metricbatcher.MetricBatcher, *eventwriter.EventWriter) {
	eventWriter := eventwriter.New(origin)
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)

	stats := runtime_stats.NewRuntimeStats(eventWriter, time.Duration(config.RuntimeStatsIntervalMilliseconds)*time.Millisecond)
	go stats.Run(stopChan)
	return metricBatcher, eventWriter
}

func storeAdapterProvider(conf *config.Config) (storeadapter.StoreAdapter, error) {
	workPool, err := workpool.NewWorkPool(conf.EtcdMaxConcurrentRequests)
	if err != nil {
		return nil, err
	}

	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: conf.EtcdUrls,
	}
	if conf.EtcdRequireTLS {
		options.IsSSL = true
		options.CertFile = conf.EtcdTLSClientConfig.CertFile
		options.KeyFile = conf.EtcdTLSClientConfig.KeyFile
		options.CAFile = conf.EtcdTLSClientConfig.CAFile
	}
	etcdAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		return nil, err
	}

	return etcdAdapter, nil
}
