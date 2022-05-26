package exporter

import (
	"github.com/gomodule/redigo/redis"
	"github.com/oliver006/redis_exporter/webank"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type ClusterExporter struct {
	*Exporter
}

func NewClusterExporter(e *Exporter, opts Options) *ClusterExporter {
	ce := &ClusterExporter{
		Exporter: e,
	}

	opts.Registry.MustRegister(ce)
	return ce
}

func (e *ClusterExporter) Describe(ch chan<- *prometheus.Desc) {

}

func (e *ClusterExporter) Collect(ch chan<- prometheus.Metric) {
	opt := e.options
	if err := e.scrapeRedisCluster(ch); err != nil {
		log.Warnf("scrape partition:%s for cluster nodes info error:%s\n", opt.Partition, err.Error())
	}
	webank.AdmCh <- struct{}{}
}

func (e *ClusterExporter) scrapeRedisCluster(ch chan<- prometheus.Metric) error {
	c, err := e.connectToRedis()
	if err != nil {
		log.Errorf("Couldn't connect to redis instance for scrape partition")
		log.Debugf("connectToRedis( %s ) err: %s", e.redisAddr, err)
		return err
	}
	defer c.Close()

	if nodes, err := redis.String(doRedisCmd(c, "CLUSTER", "NODES")); err == nil {
		webank.ClusterTopology[e.options.PartitionName] = nodes
		e.extractClusterNodesMetrics(ch, nodes)
	}

	return nil
}
