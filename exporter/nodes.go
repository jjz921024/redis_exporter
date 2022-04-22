package exporter

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	metric = "partition_node_status"
)

/**
d5fb1b0c10d8de748c9152e62b45df29056a1b35 169.254.149.66:30003@40003 master - 0 1650622974068 31 connected 10923-16383
ec624478dacdf9db0992469db6cf6107afe45860 169.254.149.66:30012@40012 slave 378e5611e0ccfc33fb0fa298bfdc6ce370f5a518 0 1650622976083 32 connected
**/
func (e *Exporter) extractClusterNodesMetrics(ch chan<- prometheus.Metric, nodes string) {
	opt := e.options
	desc := prometheus.NewDesc(metric, "", []string{"role", "status", "partition", "host"}, nil)

	infos := make(map[string]nodeInfo)

	for _, line := range strings.Split(nodes, "\n") {
		element := strings.Split(line, " ")
		if len(element) < 8 {
			continue
		}

		role := element[2]
		if !strings.Contains(role, "master") {
			// slave节点role字段存为master的id
			role = element[3]
		} else {
			// 去除 myself
			role = "master"
		}

		infos[element[0]] = nodeInfo{
			host:   strings.Split(element[1], "@")[0],
			role:   role,
			status: element[7],
		}
	}

	for _, n := range infos {
		if n.role != "master" {
			n.role = "slave of " + infos[n.role].host
		}

		lbls := []string{n.role, n.status, opt.Partition, n.host}

		m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, 0, lbls...)
		if err != nil {
			log.Printf("metric: %s  err: %s\n", metric, err.Error())
			return
		}
		ch <- m
	}
	
}


type nodeInfo struct {
	host string
	role string
	status string
}
