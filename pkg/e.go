package pkg

import (
	"fmt"
	"net/http"
	"outputGuard/global"
	. "outputGuard/logger"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type NodeCollector struct {
	IptablesCounters IptablesCounter
}

type IptablesCounter []struct {
	packetsDesc *prometheus.Desc
	bytesDesc   *prometheus.Desc
	valType     prometheus.ValueType
}

func NewNodeCollector() prometheus.Collector {
	return &NodeCollector{
		IptablesCounters: IptablesCounter{
			{
				packetsDesc: prometheus.NewDesc(
					"iptables_packets_count",
					"Iptables packets count",
					[]string{"hostname", "chain", "ip", "type"},
					nil,
				),
				bytesDesc: prometheus.NewDesc(
					"iptables_bytes_count",
					"Iptables bytes count",
					[]string{"hostname", "chain", "ip", "type"},
					nil,
				),
				valType: prometheus.GaugeValue,
			},
		},
	}
}

func (n *NodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- n.IptablesCounters[0].packetsDesc
	ch <- n.IptablesCounters[0].bytesDesc
}

func (n *NodeCollector) Collect(ch chan<- prometheus.Metric) {
	sa := time.Now()
	for ge := range global.ExporterDatas {
		if ge.ChainName == "done" {
			break
		}
		ch <- prometheus.MustNewConstMetric(
			n.IptablesCounters[0].packetsDesc,
			n.IptablesCounters[0].valType,
			ge.Packets,
			ge.Hostname,
			ge.ChainName,
			ge.Ip,
			"packets_count",
		)
		ch <- prometheus.MustNewConstMetric(
			n.IptablesCounters[0].bytesDesc,
			n.IptablesCounters[0].valType,
			ge.Bytes,
			ge.Hostname,
			ge.ChainName,
			ge.Ip,
			"bytes_count",
		)

	}
	global.RunSig <- struct{}{}

	ss := time.Since(sa)
	Logger.Info(fmt.Sprintf("exporter耗时:%v", ss))
}

func RunExporter() {
	registry := prometheus.NewRegistry()

	registry.MustRegister(NewNodeCollector())
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))
	if err := http.ListenAndServe(":9900", nil); err != nil {
		Logger.Error(fmt.Sprintf("监控程序监听端口失败!，错误信息:%s", err.Error()))
	}
}
