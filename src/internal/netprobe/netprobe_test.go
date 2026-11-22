package netprobe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNetprobeRunWithMockEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/ipcn":
			_, _ = w.Write([]byte("ip：39.180.138.19 归属地：中国 浙江 杭州 余杭 移动\n"))
		case p == "/ip138":
			_, _ = w.Write([]byte("<p>您的IP地址是：[39.180.138.19] 来自：中国浙江杭州 移动</p>"))
		case p == "/ipip":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ret":"ok","data":{"ip":"39.180.138.19","location":["中国","浙江","杭州","","移动"]}}`))
		case p == "/geoip/39.180.138.19":
			_, _ = w.Write([]byte(`{"country":"China","region":"Zhejiang","city":"Hangzhou","isp":"China Mobile","country_code":"cn"}`))
		case p == "/geoip/54.65.230.180":
			_, _ = w.Write([]byte(`{"country":"Japan","region":"Tokyo","city":"Tokyo","isp":"Amazon.com","country_code":"jp"}`))
		case p == "/geoip/13.158.209.152":
			_, _ = w.Write([]byte(`{"country":"Japan","region":"Tokyo","city":"Tokyo","isp":"Amazon.com","country_code":"jp"}`))
		case p == "/iprisk/39.180.138.19":
			_, _ = w.Write([]byte(`{"ip":"39.180.138.19","cidr":"39.180.138.0/24","asn":56041,"asOrganization":"China Mobile Communications Corporation","company_type":"isp","isResidential":true,"is_datacenter":false,"abuser_score":"0.0008 (Low)","timezone":"Asia/Shanghai"}`))
		case p == "/iprisk/54.65.230.180":
			_, _ = w.Write([]byte(`{"ip":"54.65.230.180","cidr":"54.65.0.0/16","asn":16509,"asOrganization":"Amazon.com, Inc.","company_type":"hosting","isResidential":false,"is_datacenter":true,"abuser_score":"0.0012 (Low)","timezone":"Asia/Tokyo"}`))
		case p == "/iprisk/13.158.209.152":
			_, _ = w.Write([]byte(`{"ip":"13.158.209.152","cidr":"13.158.0.0/16","asn":16509,"asOrganization":"Amazon.com, Inc.","company_type":"hosting","isResidential":false,"is_datacenter":true,"abuser_score":"0.0010 (Low)","timezone":"Asia/Tokyo"}`))
		case p == "/netease":
			w.Header().Set("cdn-user-ip", "39.180.138.19")
			w.WriteHeader(http.StatusOK)
		case p == "/bytedance":
			w.Header().Set("x-request-ip", "39.180.138.19")
			w.WriteHeader(http.StatusOK)
		case p == "/ping/200":
			w.WriteHeader(http.StatusOK)
		case p == "/ping/302":
			w.Header().Set("Location", "https://example.com/")
			w.WriteHeader(http.StatusFound)
		case p == "/ping/204":
			w.WriteHeader(http.StatusNoContent)
		case p == "/ping/404":
			w.WriteHeader(http.StatusNotFound)
		case strings.HasPrefix(p, "/cftrace/"):
			domain := strings.TrimPrefix(p, "/cftrace/")
			switch domain {
			case "www.cloudflare-cn.com", "www.qualcomm.cn":
				_, _ = w.Write([]byte("ip=39.180.138.19\nloc=CN\n"))
			case "anthropic.com", "gateway.discord.gg":
				_, _ = w.Write([]byte("ip=54.65.230.180\nloc=JP\n"))
			default:
				_, _ = w.Write([]byte("ip=13.158.209.152\nloc=JP\n"))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Closed test server to deterministically test unreachable fast-fail probe dots.
	deadSrv := httptest.NewServer(http.NewServeMux())
	deadURL := deadSrv.URL + "/timeout"
	deadSrv.Close()

	cfg := Config{
		Rounds:       12,
		IncludeRisk:  true,
		IncludeSplit: true,
		Probes: []ProbeTarget{
			{Name: "字节跳动", EnName: "ByteDance", Icon: "bytedance", Flag: "cn", Tag: "domestic", URL: srv.URL + "/ping/200"},
			{Name: "淘宝", EnName: "Taobao", Icon: "taobao", Flag: "cn", Tag: "domestic", URL: srv.URL + "/ping/302"},
			{Name: "微信", EnName: "WeChat", Icon: "weixin", Flag: "cn", Tag: "domestic", URL: srv.URL + "/ping/200"},
			{Name: "GitHub", EnName: "GitHub", Icon: "github", Tag: "international", URL: srv.URL + "/ping/404"},
			{Name: "Cloudflare", EnName: "Cloudflare", Icon: "cloudflare", Tag: "international", URL: deadURL},
			{Name: "YouTube", EnName: "YouTube", Icon: "youtube", Tag: "international", URL: srv.URL + "/ping/204"},
		},
	}

	opt := Options{
		Client: srv.Client(),
		Now:    func() time.Time { return time.Unix(1795353600, 0) },
		Endpoints: Endpoints{
			IPCN:           srv.URL + "/ipcn",
			IP138:          srv.URL + "/ip138",
			IPIP:           srv.URL + "/ipip",
			GeoIPBase:      srv.URL + "/geoip/",
			IPRiskBase:     srv.URL + "/iprisk/",
			NeteaseURL:     srv.URL + "/netease",
			BytedanceURL:   srv.URL + "/bytedance",
			CFTraceHostFmt: srv.URL + "/cftrace/%s",
		},
	}

	rep := Run(context.Background(), cfg, opt)
	if rep.Error != "" {
		t.Fatalf("unexpected report error: %s", rep.Error)
	}
	if rep.Hero.IP != "39.180.138.19" {
		t.Fatalf("Hero.IP = %q, want 39.180.138.19", rep.Hero.IP)
	}
	if rep.Hero.CountryCode != "cn" {
		t.Errorf("Hero.CountryCode = %q, want cn", rep.Hero.CountryCode)
	}
	if rep.Hero.GeoZH != "中国 浙江 杭州 余杭 移动" {
		t.Errorf("Hero.GeoZH = %q", rep.Hero.GeoZH)
	}
	if rep.Hero.PropertyZH != "家庭宽带 (ISP)" {
		t.Errorf("Hero.PropertyZH = %q", rep.Hero.PropertyZH)
	}
	if rep.Hero.Risk == nil || rep.Hero.Risk.ASN != 56041 {
		t.Errorf("Hero.Risk = %+v", rep.Hero.Risk)
	}

	if len(rep.Probes) != 6 {
		t.Fatalf("len(Probes) = %d, want 6", len(rep.Probes))
	}
	for i, p := range rep.Probes {
		if len(p.Dots) != 12 {
			t.Errorf("probe[%d] %s len(Dots) = %d, want 12", i, p.Name, len(p.Dots))
		}
		if p.Name == "Cloudflare" {
			if p.MedianMs != -1 || p.Success != 0 {
				t.Errorf("Cloudflare expected fail, got median=%d success=%d", p.MedianMs, p.Success)
			}
		} else {
			if p.MedianMs <= 0 || p.Success != 12 {
				t.Errorf("probe[%d] %s median=%d success=%d, want >0 and 12", i, p.Name, p.MedianMs, p.Success)
			}
		}
	}

	if len(rep.Exits) != 3 {
		b, _ := json.MarshalIndent(rep.Exits, "", "  ")
		t.Fatalf("len(Exits) = %d, want 3:\n%s", len(rep.Exits), string(b))
	}
	counts := map[string]int{}
	for _, ex := range rep.Exits {
		counts[ex.IP] = ex.Count
	}
	if counts["39.180.138.19"] != 4 {
		t.Errorf("CN exit site count = %d, want 4", counts["39.180.138.19"])
	}
	if counts["54.65.230.180"] != 2 {
		t.Errorf("54.65.230.180 site count = %d, want 2", counts["54.65.230.180"])
	}
	if counts["13.158.209.152"] != 12 {
		t.Errorf("13.158.209.152 site count = %d, want 12", counts["13.158.209.152"])
	}
}

func TestPickValidatedGeoRejectsMismatchedCountry(t *testing.T) {
	gd := &geoData{Country: "Japan", Region: "Tokyo", City: "Tokyo", ISP: "Amazon.com", CountryCode: "jp"}
	geo, src := pickValidatedGeo(gd, []geoCandidate{
		{ip: "13.158.209.152", geo: "中国 香港 移动", source: "iP138.com"},
	})
	if geo != "日本 Tokyo Amazon.com" || src != "GeoIP" {
		t.Fatalf("got (%q, %q), want fallback Japanese GeoIP", geo, src)
	}
}

func TestScutilProxyParsingAndLoopbackBypass(t *testing.T) {
	raw := `<dictionary> {
  ExceptionsList : <array> {
    0 : *zhihu.com
    1 : localhost
    2 : 127.*
    3 : 192.168.*
  }
  HTTPEnable : 1
  HTTPPort : 7897
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7897
  HTTPSProxy : 127.0.0.1
  SOCKSEnable : 1
  SOCKSPort : 7897
  SOCKSProxy : 127.0.0.1
}`
	cfg := parseScutilProxy(raw)
	reqDirect, _ := http.NewRequest("GET", "http://127.0.0.1:8080/test", nil)
	if u, err := ProxyFunc(reqDirect); err != nil || u != nil {
		t.Fatalf("loopback should bypass proxy, got (%v, %v)", u, err)
	}
	reqLocalhost, _ := http.NewRequest("GET", "https://localhost:3000/test", nil)
	if u, err := ProxyFunc(reqLocalhost); err != nil || u != nil {
		t.Fatalf("localhost should bypass proxy, got (%v, %v)", u, err)
	}
	reqBypass, _ := http.NewRequest("GET", "https://www.zhihu.com/explore", nil)
	if u, err := cfg.proxyFor(reqBypass.URL); err != nil || u != nil {
		t.Fatalf("*zhihu.com should bypass proxy, got (%v, %v)", u, err)
	}
	reqProxy, _ := http.NewRequest("GET", "https://claude.ai/cdn-cgi/trace", nil)
	if u, err := cfg.proxyFor(reqProxy.URL); err != nil || u == nil || u.String() != "http://127.0.0.1:7897" {
		t.Fatalf("claude.ai should use proxy http://127.0.0.1:7897, got (%v, %v)", u, err)
	}
}
