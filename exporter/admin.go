package exporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	adminUrl           = flag.String("admin-url", "http://10.107.120.69:19999", "WE-REDIS ADMIN URL OF WEBANK")
	assembleInfoPath   = flag.String("assmeble-info-path", "/weredis/clusterinfo/v1/getAssembleInfo", "assemble info path")

	enableReport	   = flag.Bool("enable-report-admin", true, "enable report cluster topo to admin")
	reportTopoPath     = flag.String("report-topo-path", "/weredis/cluster/node/status/v1/", "report cluster topology path")

	enableStatistic    = flag.Bool("enbale-statistic", true, "enable statistic subsyster used capacity")
	execHour           = *flag.Int("exec-hour", 1, "exec statistic hour in day")
	sampleRate         = *flag.Float64("smapleRate", 0.1, "subsystem capacity sample rate [0, 1]")    
	scanCount 		   = flag.Int("scan-count", 1000, "count of per scan")
	itemsThreshold     = flag.Int("itemThreshold", 1000, "the items threshold of Big nested data types")
	memoryThreshold    = flag.Int("memoryThreshold", 10000, "the memory threshold of Big KeyValue")
	ttlThreshold       = flag.Int("ttlThreshold", 1000000, "the ttl threshold of key")

	scrapeLimit        = flag.Int("tps", 10, "scrape tps limit")
	expireSecond       = flag.Int("expire-second", 1 * 60 * 60, "cluster info expire time")

	host 			   = *flag.String("exporter-host", "127.0.0.1", "exporter host")

	// 维护所有集群名和实例的关系
	allClustersInfo = sync.Map{}

	// 上报admin的触发channel
	admCh = make(chan *ClusterExporter)
	client *http.Client
)

// 创建与admin相关的定时任务
// 1. 上报集群拓扑信息
// 2. 开启子系统key扫描任务
func init() {
	tr := &http.Transport{
		MaxIdleConns: 10,
	}
	client = &http.Client{
		Transport: tr,
		Timeout:   3 * time.Second,
	}

	if *enableReport {
		log.Println("enable report cluster topology info to admin")
		go func() {
			for e := range admCh {
				reportClusterTopology(e.clusterName, e.clusterTopology)			
			}
		}()		
	}

	if *enableStatistic {
		// TODO: 
		/* t := time.Now()
		if (t.Hour() > execHour) {
			t.Add(24 * time.Hour)	
		}
		delay := time.Until(t) */

		delay := time.Second * 5
		log.Printf("enable statistics subsystem keys, after:%v\n", delay)

		time.AfterFunc(delay, func() {
			// 设置每天定时执行一次
			go func() {
				ticker := time.NewTicker(delay) //24 * time.Hour
				defer ticker.Stop()
				for range ticker.C {
					sampleStatClusterUsage()
				}
			} ()

			// 首次执行
			sampleStatClusterUsage()
		})
	}
}

// 根据集群名获取分区信息, 并缓存一段时间
func getClusterInfo(clusterName string) (*ClusterInfo, error) {
	clusterInfo, exist := allClustersInfo.Load(clusterName)
	if !exist {
		value, err := getAssembleInfo(clusterName)
		if err != nil {
			return nil, fmt.Errorf("get cluster info from admin err:%s", err.Error())
		}
		clusterInfo = value
		allClustersInfo.Store(clusterName, clusterInfo)
		// 缓存过期时间
		time.AfterFunc(time.Duration(*expireSecond) * time.Second, func() {
			allClustersInfo.Delete(clusterName)
		})
	}

	ci, ok := clusterInfo.(*ClusterInfo)
	if !ok {
		return nil, errors.New("get cluster info type assert err")
	}
	return ci, nil
} 

func getAssembleInfo(clusterName string) (*ClusterInfo, error) {
	req, err := http.NewRequest("GET", *adminUrl+*assembleInfoPath, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("clusterName", clusterName)
	q.Set("componentRole", "exporter")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("get assemble info err:%s\n", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, errors.New("stat request fail, code:" + strconv.Itoa(resp.StatusCode))
	}

	result := &assembleResponse{}
	err = json.Unmarshal(body, result)
	if err != nil {
		return nil, err
	} else if result.Code != "0" {
		return nil, errors.New("fail: " + result.Msg)
	}

	// 反序列成功后，设置集群名字段
	result.ResultData.Name = clusterName
	return &result.ResultData, nil
}

type response struct {
	Code       string      `json:"code"`
	Msg        string      `json:"msg"`
	Page       *string     `json:"page"`
	Others     interface{} `json:"others"`
}

type assembleResponse struct {
	response
	ResultData ClusterInfo `json:"resultData"`
}

type reportResponse struct {
	response
	ResultData interface{} `json:"resultData"`
}

// RPD|RPD_GENERAL_REDIS_NODESET_1_CACHE|1|169.254.149.66:30001,169.254.149.66:30002,169.254.149.66:30003
func (c *ClusterInfo) UnmarshalJSON(data []byte) error {
	content := strings.Trim(string(data), "\"")
	// 处理每个分区的数据
	for _, s := range strings.Split(content, ";") {
		// 取num和host, 包含全部主从节点
		split := strings.Split(s, "|")
		if len(split) != 4 {
			continue
		}

		p := PartitionInfo{
			Name: split[1],
			Num:  split[2],
		}

		hosts := strings.Split(split[3], ",")
		nodes := make([]NodeInfo, len(hosts))
		for i, h := range hosts {
			nodes[i] = NodeInfo{
				PartitionNum: p.Num,
				PartitionName: p.Name,
				Host:         h,
			}
		}
		p.Nodes = nodes
		c.Partitions = append(c.Partitions, p)
	}
	return nil
}

// 上报集群拓扑信息到admin
func reportClusterTopology(clusterName string, clusterTopology sync.Map) {
	// partition <--> nodes
	topo := make(map[string]interface{}, 2)
	partitions := make(map[string][]NodeTopo)

	topo["clusterName"] = clusterName
	topo["partitions"] = partitions

	clusterTopology.Range(func(key, value interface{}) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		nodes, ok := value.(string)
		if !ok {
			return true
		}

		arr := strings.Split(nodes, "\n")
		n := []NodeTopo{}

		for _, line := range arr {
			element := strings.Split(line, " ")
			if len(element) < 8 {
				//log.Printf("cluster info invalid:%s\n", line)
				continue
			}

			id := element[0]

			host := strings.Split(element[1], "@")[0]
			s := strings.Split(host, ":")
			if len(s) != 2 {
				log.Printf("invalid host:%s\n", s)
				continue
			}
			ip := s[0]
			port, err := strconv.Atoi(s[1])
			if err != nil {
				log.Printf("parse port:%s err:%s\n", s[1], err.Error())
				continue
			}

			role := element[2]
			slaveOf := ""
			if !strings.Contains(role, "master") {
				role = "slave"
				slaveOf = element[3]
			} else {
				role = "master"
			}

			status := "success"
			if strings.Contains(element[2], "fail") || element[7] != "connected" {
				status = "fail"
			}

			n = append(n, NodeTopo{
				Id: id,
				Ip: ip,
				Port: port,
				Role:   role,
				Status: status,
				SlaveOf: slaveOf,
			})
		}
		partitions[name] = n
		return true
	})

	body, err := json.Marshal(topo)
	if err != nil {
		log.Printf("json encode err:%s", err.Error())
		return
	}

	//log.Printf("%s", string(b))
	req, err := http.NewRequest("POST", *adminUrl+*reportTopoPath, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	if err != nil {
		log.Printf("json encode err:%s", err.Error())
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("report cluster topo err:%s\n", err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("report topo err:%s\n", err.Error())
		return
	} else if resp.StatusCode != 200 {
		log.Printf("report topo reponse fail, code:%d, conetnt:%s\n", resp.StatusCode, string(respBody))
		return
	}

	result := &reportResponse{}
	err = json.Unmarshal(respBody, result)
	if err != nil {
		log.Printf("report topo response invalid err:%s, resp:%s\n", err.Error(), string(respBody))
		return
	} else if result.Code != "0" {
		log.Printf("report topo response code:%s, content%s\n", result.Code, string(respBody))
		return
	}
}