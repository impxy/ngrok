package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/impxy/ngrok/conn"
	"github.com/impxy/ngrok/log"
	gometrics "github.com/rcrowley/go-metrics"
)

var metrics Metrics

func init() {
	keenApiKey := os.Getenv("KEEN_API_KEY")

	if keenApiKey != "" {
		metrics = NewKeenIoMetrics(60 * time.Second)
	} else {
		metrics = NewLocalMetrics(30 * time.Second)
	}
}

type Metrics interface {
	log.Logger
	OpenConnection(*Tunnel, conn.Conn)
	CloseConnection(*Tunnel, conn.Conn, time.Time, int64, int64)
	OpenTunnel(*Tunnel)
	CloseTunnel(*Tunnel)
}

type LocalMetrics struct {
	log.Logger
	reportInterval time.Duration
	windowsCounter gometrics.Counter
	linuxCounter   gometrics.Counter
	osxCounter     gometrics.Counter
	otherCounter   gometrics.Counter

	tunnelMeter        gometrics.Meter
	tcpTunnelMeter     gometrics.Meter
	httpTunnelMeter    gometrics.Meter
	connMeter          gometrics.Meter
	lostHeartbeatMeter gometrics.Meter

	connTimer gometrics.Timer

	bytesInCount  gometrics.Counter
	bytesOutCount gometrics.Counter

	//===pxy修改===
	//实际数量/当前数量（原始的是不会减少的数量）
	currWindowsCounter		gometrics.Counter
	currLinuxCounter		gometrics.Counter
	currOsxCounter			gometrics.Counter
	currOtherCounter		gometrics.Counter
	
	currTunnelMeter        gometrics.Meter
	currTcpTunnelMeter     gometrics.Meter
	currHttpTunnelMeter    gometrics.Meter
	currConnMeter          gometrics.Meter

	//协议细化
	udpTunnelMeter			gometrics.Meter
	currUdpTunnelMeter		gometrics.Meter
	httpsTunnelMeter		gometrics.Meter
	currHttpsTunnelMeter	gometrics.Meter

	//客户端url
	clientUrlList			[]string		//客户端url列表（不会减少的）
	currClientUrlList		[]string		//当前连接的客户端url列表

	//===pxy修改===

	/*
	   tunnelGauge gometrics.Gauge
	   tcpTunnelGauge gometrics.Gauge
	   connGauge gometrics.Gauge
	*/
}

func NewLocalMetrics(reportInterval time.Duration) *LocalMetrics {
	metrics := LocalMetrics{
		Logger:         log.NewPrefixLogger("metrics"),
		reportInterval: reportInterval,
		windowsCounter: gometrics.NewCounter(),
		linuxCounter:   gometrics.NewCounter(),
		osxCounter:     gometrics.NewCounter(),
		otherCounter:   gometrics.NewCounter(),

		tunnelMeter:        gometrics.NewMeter(),
		tcpTunnelMeter:     gometrics.NewMeter(),
		httpTunnelMeter:    gometrics.NewMeter(),
		connMeter:          gometrics.NewMeter(),
		lostHeartbeatMeter: gometrics.NewMeter(),

		connTimer: gometrics.NewTimer(),

		bytesInCount:  gometrics.NewCounter(),
		bytesOutCount: gometrics.NewCounter(),


		//===pxy修改===
		currWindowsCounter:		gometrics.NewCounter(),
		currLinuxCounter:		gometrics.NewCounter(),
		currOsxCounter:			gometrics.NewCounter(),
		currOtherCounter:		gometrics.NewCounter(),

		currTunnelMeter:        gometrics.NewMeter(),
		currTcpTunnelMeter:     gometrics.NewMeter(),
		currHttpTunnelMeter:    gometrics.NewMeter(),
		currConnMeter:          gometrics.NewMeter(),

		
		udpTunnelMeter:			gometrics.NewMeter(),
		currUdpTunnelMeter:		gometrics.NewMeter(),
		httpsTunnelMeter:		gometrics.NewMeter(),
		currHttpsTunnelMeter:	gometrics.NewMeter(),

		clientUrlList:			[]string,
		currClientUrlList:		[]string,
		//===pxy修改===

		/*
		   metrics.tunnelGauge = gometrics.NewGauge(),
		   metrics.tcpTunnelGauge = gometrics.NewGauge(),
		   metrics.connGauge = gometrics.NewGauge(),
		*/
	}

	go metrics.Report()

	return &metrics
}

func (m *LocalMetrics) OpenTunnel(t *Tunnel) {
	m.tunnelMeter.Mark(1)

	switch t.ctl.auth.OS {
	case "windows":
		m.windowsCounter.Inc(1)
	case "linux":
		m.linuxCounter.Inc(1)
	case "darwin":
		m.osxCounter.Inc(1)
	default:
		m.otherCounter.Inc(1)
	}

	switch t.req.Protocol {
	case "tcp":
		m.tcpTunnelMeter.Mark(1)
	case "http":
		m.httpTunnelMeter.Mark(1)
	case "udp":
		m.udpTunnelMeter.Mark(1)
	case "https":
		m.httpsTunnelMeter.Mark(1)
	}

	//===pxy修改===
	m.currTunnelMeter.Mark(1)

	switch t.ctl.auth.OS {
	case "windows":
		m.currWindowsCounter.Inc(1)
	case "linux":
		m.currLinuxCounter.Inc(1)
	case "darwin":
		m.currOsxCounter.Inc(1)
	default:
		m.currOtherCounter.Inc(1)
	}

	switch t.req.Protocol {
	case "tcp":
		m.currTcpTunnelMeter.Mark(1)
	case "http":
		m.currHttpTunnelMeter.Mark(1)
	case "udp":
		m.currUdpTunnelMeter.Mark(1)
	case "https":
		m.currHttpsTunnelMeter.Mark(1)
	}

	m.clientUrlList = append(m.clientUrlList, t.url)
	m.currClientUrlList = append(m.currClientUrlList, t.url)
	//===pxy修改===
}

func (m *LocalMetrics) CloseTunnel(t *Tunnel) {

	//===pxy修改===
	m.currTunnelMeter.Mark(-1)

	switch t.ctl.auth.OS {
	case "windows":
		m.currWindowsCounter.Inc(-1)
	case "linux":
		m.currLinuxCounter.Inc(-1)
	case "darwin":
		m.currOsxCounter.Inc(-1)
	default:
		m.currOtherCounter.Inc(-1)
	}

	switch t.req.Protocol {
	case "tcp":
		m.currTcpTunnelMeter.Mark(-1)
	case "http":
		m.currHttpTunnelMeter.Mark(-1)
	case "udp":
		m.currUdpTunnelMeter.Mark(-1)
	case "https":
		m.currHttpsTunnelMeter.Mark(-1)
	}


	//===pxy修改===

}

func (m *LocalMetrics) OpenConnection(t *Tunnel, c conn.Conn) {
	m.connMeter.Mark(1)

	//===pxy修改===
	m.currConnMeter.Mark(1)
	//===pxy修改===

}

func (m *LocalMetrics) CloseConnection(t *Tunnel, c conn.Conn, start time.Time, bytesIn, bytesOut int64) {
	m.bytesInCount.Inc(bytesIn)
	m.bytesOutCount.Inc(bytesOut)

	//===pxy修改===
	m.currConnMeter.Mark(-1)
	//===pxy修改===
}

func (m *LocalMetrics) Report() {
	m.Info("Reporting every %d seconds", int(m.reportInterval.Seconds()))

	for {
		time.Sleep(m.reportInterval)
		buffer, err := json.Marshal(map[string]interface{}{
			"windows":               m.windowsCounter.Count(),
			"linux":                 m.linuxCounter.Count(),
			"osx":                   m.osxCounter.Count(),
			"other":                 m.otherCounter.Count(),
			"httpTunnelMeter.count": m.httpTunnelMeter.Count(),
			"tcpTunnelMeter.count":  m.tcpTunnelMeter.Count(),
			"tunnelMeter.count":     m.tunnelMeter.Count(),
			"tunnelMeter.m1":        m.tunnelMeter.Rate1(),
			"connMeter.count":       m.connMeter.Count(),
			"connMeter.m1":          m.connMeter.Rate1(),
			"bytesIn.count":         m.bytesInCount.Count(),
			"bytesOut.count":        m.bytesOutCount.Count(),

			"currWindows":					m.currWindowsCounter.Count(),
			"currLinux":					m.currLinuxCounter.Count(),
			"currOsx":						m.currOsxCounter.Count(),
			"currOther":					m.currOtherCounter.Count(),
			"currHttpTunnelMeter.count":	m.currHttpTunnelMeter.Count(),
			"currTcpTunnelMeter.count":		m.currTcpTunnelMeter.Count(),
			"currTunnelMeter.count":		m.currTunnelMeter.Count(),
			"currConnMeter.count":			m.currConnMeter.Count(),

			"udpTunnelMeter.count":			m.udpTunnelMeter.Count(),
			"currUdpTunnelMeter.count":		m.currUdpTunnelMeter.Count(),
			"httpsTunnelMeter.count":		m.httpsTunnelMeter.Count(),
			"currHttpsTunnelMeter.count":	m.currHttpsTunnelMeter.Count(),

			"clientUrlList":				m.clientUrlList,
			"currClientUrlList":			m.currClientUrlList

		})

		if err != nil {
			m.Error("Failed to serialize metrics: %v", err)
			continue
		}

		m.Info("Reporting: %s", buffer)
	}
}

type KeenIoMetric struct {
	Collection string
	Event      interface{}
}

type KeenIoMetrics struct {
	log.Logger
	ApiKey       string
	ProjectToken string
	HttpClient   http.Client
	Metrics      chan *KeenIoMetric
}

func NewKeenIoMetrics(batchInterval time.Duration) *KeenIoMetrics {
	k := &KeenIoMetrics{
		Logger:       log.NewPrefixLogger("metrics"),
		ApiKey:       os.Getenv("KEEN_API_KEY"),
		ProjectToken: os.Getenv("KEEN_PROJECT_TOKEN"),
		Metrics:      make(chan *KeenIoMetric, 1000),
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				k.Error("KeenIoMetrics failed: %v", r)
			}
		}()

		batch := make(map[string][]interface{})
		batchTimer := time.Tick(batchInterval)

		for {
			select {
			case m := <-k.Metrics:
				list, ok := batch[m.Collection]
				if !ok {
					list = make([]interface{}, 0)
				}
				batch[m.Collection] = append(list, m.Event)

			case <-batchTimer:
				// no metrics to report
				if len(batch) == 0 {
					continue
				}

				payload, err := json.Marshal(batch)
				if err != nil {
					k.Error("Failed to serialize metrics payload: %v, %v", batch, err)
				} else {
					for key, val := range batch {
						k.Debug("Reporting %d metrics for %s", len(val), key)
					}

					k.AuthedRequest("POST", "/events", bytes.NewReader(payload))
				}
				batch = make(map[string][]interface{})
			}
		}
	}()

	return k
}

func (k *KeenIoMetrics) AuthedRequest(method, path string, body *bytes.Reader) (resp *http.Response, err error) {
	path = fmt.Sprintf("https://api.keen.io/3.0/projects/%s%s", k.ProjectToken, path)
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return
	}

	req.Header.Add("Authorization", k.ApiKey)

	if body != nil {
		req.Header.Add("Content-Type", "application/json")
		req.ContentLength = int64(body.Len())
	}

	requestStartAt := time.Now()
	resp, err = k.HttpClient.Do(req)

	if err != nil {
		k.Error("Failed to send metric event to keen.io %v", err)
	} else {
		k.Info("keen.io processed request in %f sec", time.Since(requestStartAt).Seconds())
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			bytes, _ := ioutil.ReadAll(resp.Body)
			k.Error("Got %v response from keen.io: %s", resp.StatusCode, bytes)
		}
	}

	return
}

func (k *KeenIoMetrics) OpenConnection(t *Tunnel, c conn.Conn) {
}

func (k *KeenIoMetrics) CloseConnection(t *Tunnel, c conn.Conn, start time.Time, in, out int64) {
	event := struct {
		Keen               KeenStruct `json:"keen"`
		OS                 string
		ClientId           string
		Protocol           string
		Url                string
		User               string
		Version            string
		Reason             string
		HttpAuth           bool
		Subdomain          bool
		TunnelDuration     float64
		ConnectionDuration float64
		BytesIn            int64
		BytesOut           int64
	}{
		Keen: KeenStruct{
			Timestamp: start.UTC().Format("2006-01-02T15:04:05.000Z"),
		},
		OS:                 t.ctl.auth.OS,
		ClientId:           t.ctl.id,
		Protocol:           t.req.Protocol,
		Url:                t.url,
		User:               t.ctl.auth.User,
		Version:            t.ctl.auth.MmVersion,
		HttpAuth:           t.req.HttpAuth != "",
		Subdomain:          t.req.Subdomain != "",
		TunnelDuration:     time.Since(t.start).Seconds(),
		ConnectionDuration: time.Since(start).Seconds(),
		BytesIn:            in,
		BytesOut:           out,
	}

	k.Metrics <- &KeenIoMetric{Collection: "CloseConnection", Event: event}
}

func (k *KeenIoMetrics) OpenTunnel(t *Tunnel) {
}

type KeenStruct struct {
	Timestamp string `json:"timestamp"`
}

func (k *KeenIoMetrics) CloseTunnel(t *Tunnel) {
	event := struct {
		Keen      KeenStruct `json:"keen"`
		OS        string
		ClientId  string
		Protocol  string
		Url       string
		User      string
		Version   string
		Reason    string
		Duration  float64
		HttpAuth  bool
		Subdomain bool
	}{
		Keen: KeenStruct{
			Timestamp: t.start.UTC().Format("2006-01-02T15:04:05.000Z"),
		},
		OS:       t.ctl.auth.OS,
		ClientId: t.ctl.id,
		Protocol: t.req.Protocol,
		Url:      t.url,
		User:     t.ctl.auth.User,
		Version:  t.ctl.auth.MmVersion,
		//Reason: reason,
		Duration:  time.Since(t.start).Seconds(),
		HttpAuth:  t.req.HttpAuth != "",
		Subdomain: t.req.Subdomain != "",
	}

	k.Metrics <- &KeenIoMetric{Collection: "CloseTunnel", Event: event}
}
