package netprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultRounds         = 12
	maxRounds             = 20
	browserUA             = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
	defaultIPCNURL        = "https://my.ip.cn/"
	defaultIP138URL       = "https://2026.ip138.com/"
	defaultIPIPURL        = "https://myip.ipip.net/json"
	defaultGeoIPBase      = "https://ip.net.coffee/api/geoip/"
	defaultIPRiskBase     = "https://ip.net.coffee/api/iprisk/"
	defaultGeoFallback1   = "https://api.ip.sb/geoip/"
	defaultGeoFallback2   = "https://ipwho.is/"
	defaultFallbackTrace  = "https://www.cloudflare-cn.com/cdn-cgi/trace"
	defaultNeteaseURL     = "https://necaptcha.nosdn.127.net/ab7f4275c1744aa28e0a8f3a1c58c532.png"
	defaultBytedanceURL   = "https://perfops.byte-test.com/500b-bench.jpg"
	defaultCFTraceHostFmt = "https://%s/cdn-cgi/trace"
)

var (
	ipv4Re   = regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)
	ip138Geo = regexp.MustCompile(`来自：([^<\r\n]+)`)
	ipcnGeo  = regexp.MustCompile(`归属地：([^\r\n]+)`)
)

// Config configures one network & IP probe run.
type Config struct {
	Rounds       int           `json:"rounds,omitempty"`
	IncludeRisk  bool          `json:"includeRisk,omitempty"`
	IncludeSplit bool          `json:"includeSplit,omitempty"`
	Probes       []ProbeTarget `json:"probes,omitempty"`
	Splits       []SplitTarget `json:"splits,omitempty"`
}

// ProbeTarget defines a single HTTP latency probe endpoint.
type ProbeTarget struct {
	Name   string `json:"name"`
	EnName string `json:"enName,omitempty"`
	Icon   string `json:"icon,omitempty"`
	Flag   string `json:"flag,omitempty"`
	Tag    string `json:"tag,omitempty"` // domestic | international
	URL    string `json:"url"`
}

// SplitTarget defines a single website split-routing exit IP probe.
type SplitTarget struct {
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	Type     string `json:"type"`               // domestic | international
	Category string `json:"category,omitempty"` // social | ai | crypto | tools | dev | speed | static
	Method   string `json:"method"`             // cftrace | netease | bytedance
	Domain   string `json:"domain,omitempty"`
	URL      string `json:"url,omitempty"`
}

// IPRisk mirrors the IP risk / ASN classification returned by /api/iprisk/{ip}.
type IPRisk struct {
	IP             string `json:"ip"`
	CIDR           string `json:"cidr,omitempty"`
	ASN            int    `json:"asn,omitempty"`
	ASOrganization string `json:"asOrganization,omitempty"`
	CompanyType    string `json:"company_type,omitempty"`
	CompanyName    string `json:"company_name,omitempty"`
	IsResidential  *bool  `json:"isResidential,omitempty"`
	IsDatacenter   bool   `json:"is_datacenter,omitempty"`
	IsVPN          bool   `json:"is_vpn,omitempty"`
	IsProxy        bool   `json:"is_proxy,omitempty"`
	IsTor          bool   `json:"is_tor,omitempty"`
	IsCrawler      bool   `json:"is_crawler,omitempty"`
	IsAbuser       bool   `json:"is_abuser,omitempty"`
	IsMobile       bool   `json:"is_mobile,omitempty"`
	AbuserScore    string `json:"abuser_score,omitempty"`
	DatacenterName string `json:"datacenter_name,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	RDNS           string `json:"rdns,omitempty"`
	Country        string `json:"country,omitempty"`
	CountryCode    string `json:"countryCode,omitempty"`
	Region         string `json:"region,omitempty"`
	City           string `json:"city,omitempty"`
}

// HeroIP describes the primary domestic/local egress IPv4 and validated geolocation.
type HeroIP struct {
	IP          string  `json:"ip"`
	CountryCode string  `json:"countryCode,omitempty"`
	Country     string  `json:"country,omitempty"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	ISP         string  `json:"isp,omitempty"`
	GeoZH       string  `json:"geoZh,omitempty"`
	GeoEN       string  `json:"geoEn,omitempty"`
	Source      string  `json:"source,omitempty"`
	PropertyZH  string  `json:"propertyZh,omitempty"`
	PropertyEN  string  `json:"propertyEn,omitempty"`
	Risk        *IPRisk `json:"risk,omitempty"`
}

// ProbeResult captures the 12-round RTT summary for one probe target.
type ProbeResult struct {
	Name     string `json:"name"`
	EnName   string `json:"enName,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Flag     string `json:"flag,omitempty"`
	Tag      string `json:"tag,omitempty"`
	URL      string `json:"url"`
	MedianMs int    `json:"medianMs"`
	MinMs    int    `json:"minMs"`
	MaxMs    int    `json:"maxMs"`
	Success  int    `json:"success"`
	Total    int    `json:"total"`
	Dots     []int  `json:"dots"`
}

// ExitIP summarizes one unique exit IP seen across split-routing tests.
type ExitIP struct {
	IP          string   `json:"ip"`
	CountryCode string   `json:"countryCode,omitempty"`
	Country     string   `json:"country,omitempty"`
	Region      string   `json:"region,omitempty"`
	City        string   `json:"city,omitempty"`
	ISP         string   `json:"isp,omitempty"`
	GeoZH       string   `json:"geoZh,omitempty"`
	GeoEN       string   `json:"geoEn,omitempty"`
	PropertyZH  string   `json:"propertyZh,omitempty"`
	PropertyEN  string   `json:"propertyEn,omitempty"`
	Count       int      `json:"count"`
	Sites       []string `json:"sites,omitempty"`
	Risk        *IPRisk  `json:"risk,omitempty"`
}

// SplitResult holds the observed exit IP and location for one tested website.
type SplitResult struct {
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Type        string `json:"type"`
	Category    string `json:"category,omitempty"`
	IP          string `json:"ip,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	GeoZH       string `json:"geoZh,omitempty"`
	GeoEN       string `json:"geoEn,omitempty"`
	LatencyMs   int    `json:"latencyMs"`
	Status      string `json:"status"` // ok | error
	Error       string `json:"error,omitempty"`
}

// Report is the complete snapshot returned to scripts as ctx.netprobe.
type Report struct {
	FetchedAt int64         `json:"fetchedAt"`
	Stale     bool          `json:"stale"`
	Error     string        `json:"error,omitempty"`
	Hero      HeroIP        `json:"hero"`
	Probes    []ProbeResult `json:"probes"`
	Exits     []ExitIP      `json:"exits"`
	Splits    []SplitResult `json:"splits"`
}

// Endpoints allows tests to substitute local httptest endpoints for external services.
type Endpoints struct {
	IPCN           string
	IP138          string
	IPIP           string
	GeoIPBase      string
	IPRiskBase     string
	GeoFallback1   string
	GeoFallback2   string
	FallbackTrace  string
	NeteaseURL     string
	BytedanceURL   string
	CFTraceHostFmt string
}

// Options configures execution dependencies for Run.
type Options struct {
	Client    *http.Client
	Now       func() time.Time
	Endpoints Endpoints
}

// DefaultProbes returns the 6 default connectivity latency targets matching ip.net.coffee.
func DefaultProbes() []ProbeTarget {
	return []ProbeTarget{
		{Name: "字节跳动", EnName: "ByteDance", Icon: "bytedance", Flag: "cn", Tag: "domestic", URL: "https://perfops.byte-test.com/500b-bench.jpg"},
		{Name: "淘宝", EnName: "Taobao", Icon: "taobao", Flag: "cn", Tag: "domestic", URL: "https://www.taobao.com/favicon.ico"},
		{Name: "微信", EnName: "WeChat", Icon: "weixin", Flag: "cn", Tag: "domestic", URL: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico"},
		{Name: "GitHub", EnName: "GitHub", Icon: "github", Flag: "", Tag: "international", URL: "https://github.com/generate_204"},
		{Name: "Cloudflare", EnName: "Cloudflare", Icon: "cloudflare", Flag: "", Tag: "international", URL: "https://1.1.1.1/cdn-cgi/trace"},
		{Name: "YouTube", EnName: "YouTube", Icon: "youtube", Flag: "", Tag: "international", URL: "https://www.youtube.com/generate_204"},
	}
}

// DefaultSplits returns the 18 default website split-routing test targets.
func DefaultSplits() []SplitTarget {
	return []SplitTarget{
		// Domestic (4)
		{Name: "网易", Icon: "netease", Type: "domestic", Method: "netease", URL: defaultNeteaseURL},
		{Name: "字节跳动", Icon: "bytedance", Type: "domestic", Method: "bytedance", URL: defaultBytedanceURL},
		{Name: "Cloudflare中国", Icon: "cloudflare", Type: "domestic", Method: "cftrace", Domain: "www.cloudflare-cn.com"},
		{Name: "高通中国", Icon: "qualcomm", Type: "domestic", Method: "cftrace", Domain: "www.qualcomm.cn"},

		// International Social (4)
		{Name: "discord.com", Icon: "discord", Type: "international", Category: "social", Method: "cftrace", Domain: "gateway.discord.gg"},
		{Name: "x.com", Icon: "x", Type: "international", Category: "social", Method: "cftrace", Domain: "x.com"},
		{Name: "medium.com", Icon: "medium", Type: "international", Category: "social", Method: "cftrace", Domain: "medium.com"},
		{Name: "quora.com", Icon: "quora", Type: "international", Category: "social", Method: "cftrace", Domain: "quora.com"},

		// International AI (5)
		{Name: "anthropic.com", Icon: "anthropic", Type: "international", Category: "ai", Method: "cftrace", Domain: "anthropic.com"},
		{Name: "claude.ai", Icon: "claude", Type: "international", Category: "ai", Method: "cftrace", Domain: "claude.ai"},
		{Name: "chatgpt.com", Icon: "chatgpt", Type: "international", Category: "ai", Method: "cftrace", Domain: "chatgpt.com"},
		{Name: "openai.com", Icon: "openai", Type: "international", Category: "ai", Method: "cftrace", Domain: "openai.com"},
		{Name: "grok.com", Icon: "grok", Type: "international", Category: "ai", Method: "cftrace", Domain: "grok.com"},

		// International Crypto (2)
		{Name: "coinbase.com", Icon: "coinbase", Type: "international", Category: "crypto", Method: "cftrace", Domain: "coinbase.com"},
		{Name: "binance.com", Icon: "binance", Type: "international", Category: "crypto", Method: "cftrace", Domain: "www.binance.info"},

		// International Tools/Dev/CDN (3)
		{Name: "zoom.us", Icon: "zoom", Type: "international", Category: "tools", Method: "cftrace", Domain: "st1.zoom.us"},
		{Name: "cloudflare.com", Icon: "cloudflare", Type: "international", Category: "speed", Method: "cftrace", Domain: "www.cloudflare.com"},
		{Name: "npm registry", Icon: "npmjs", Type: "international", Category: "static", Method: "cftrace", Domain: "registry.npmjs.org"},
	}
}

func resolveEndpoints(ep Endpoints) Endpoints {
	if ep.IPCN == "" {
		ep.IPCN = defaultIPCNURL
	}
	if ep.IP138 == "" {
		ep.IP138 = defaultIP138URL
	}
	if ep.IPIP == "" {
		ep.IPIP = defaultIPIPURL
	}
	if ep.GeoIPBase == "" {
		ep.GeoIPBase = defaultGeoIPBase
	}
	if ep.IPRiskBase == "" {
		ep.IPRiskBase = defaultIPRiskBase
	}
	if ep.GeoFallback1 == "" {
		ep.GeoFallback1 = defaultGeoFallback1
	}
	if ep.GeoFallback2 == "" {
		ep.GeoFallback2 = defaultGeoFallback2
	}
	if ep.FallbackTrace == "" {
		ep.FallbackTrace = defaultFallbackTrace
	}
	if ep.NeteaseURL == "" {
		ep.NeteaseURL = defaultNeteaseURL
	}
	if ep.BytedanceURL == "" {
		ep.BytedanceURL = defaultBytedanceURL
	}
	if ep.CFTraceHostFmt == "" {
		ep.CFTraceHostFmt = defaultCFTraceHostFmt
	}
	return ep
}

// Run performs concurrent Hero IP detection, 12-round HTTP connectivity probing,
// optional website split-routing detection, and GeoIP/IPRisk enrichment.
func Run(ctx context.Context, cfg Config, opt Options) Report {
	now := time.Now
	if opt.Now != nil {
		now = opt.Now
	}
	client := opt.Client
	if client == nil {
		client = NewClient(10 * time.Second)
	}
	ep := resolveEndpoints(opt.Endpoints)

	rounds := cfg.Rounds
	if rounds <= 0 {
		rounds = defaultRounds
	}
	if rounds > maxRounds {
		rounds = maxRounds
	}

	probes := cfg.Probes
	if len(probes) == 0 {
		probes = DefaultProbes()
	}
	splits := cfg.Splits
	if len(splits) == 0 && cfg.IncludeSplit {
		splits = DefaultSplits()
	}

	rep := Report{
		FetchedAt: now().UnixMilli(),
		Probes:    make([]ProbeResult, len(probes)),
	}
	if cfg.IncludeSplit {
		rep.Splits = make([]SplitResult, len(splits))
	}

	var (
		ip138Cand *geoCandidate
		ipCNCand  *geoCandidate
		ipipCand  *geoCandidate
		traceCand *geoCandidate
		wg        sync.WaitGroup
	)

	// Phase 1: Domestic Hero IP queries in parallel.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var hwg sync.WaitGroup
		hwg.Add(3)
		go func() {
			defer hwg.Done()
			ip138Cand = fetchIP138(ctx, client, ep.IP138)
		}()
		go func() {
			defer hwg.Done()
			ipCNCand = fetchIPCN(ctx, client, ep.IPCN)
		}()
		go func() {
			defer hwg.Done()
			ipipCand = fetchIPIP(ctx, client, ep.IPIP)
		}()
		hwg.Wait()
		if ip138Cand == nil && ipCNCand == nil && ipipCand == nil && ep.FallbackTrace != "" {
			if tr, err := fetchCFTraceURL(ctx, client, ep.FallbackTrace); err == nil && tr.ip != "" {
				traceCand = &geoCandidate{ip: tr.ip, source: "Cloudflare CN"}
			}
		}
	}()

	// Phase 2: 6-target keep-alive HTTP latency probes.
	wg.Add(1)
	go func() {
		defer wg.Done()
		probeClient := noRedirectClient(client)
		var pwg sync.WaitGroup
		for i, t := range probes {
			pwg.Add(1)
			go func(idx int, target ProbeTarget) {
				defer pwg.Done()
				rep.Probes[idx] = runProbeTarget(ctx, probeClient, target, rounds)
			}(i, t)
		}
		pwg.Wait()
	}()

	// Phase 3: 18-site split-routing worker pool (concurrency 12).
	if cfg.IncludeSplit && len(splits) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSplitTests(ctx, client, ep, splits, rep.Splits)
		}()
	}

	wg.Wait()

	// Choose primary Hero IP and Geo candidates.
	var heroIP string
	var heroSource string
	var heroCands []geoCandidate
	switch {
	case ip138Cand != nil && ipCNCand != nil:
		if ip138Cand.ip == ipCNCand.ip {
			heroIP = ip138Cand.ip
			heroSource = "iP138.com / IP.cn"
			heroCands = append(heroCands, *ip138Cand, *ipCNCand)
		} else {
			heroIP = ip138Cand.ip
			heroSource = ip138Cand.source
			heroCands = append(heroCands, *ip138Cand)
		}
	case ip138Cand != nil:
		heroIP = ip138Cand.ip
		heroSource = ip138Cand.source
		heroCands = append(heroCands, *ip138Cand)
	case ipCNCand != nil:
		heroIP = ipCNCand.ip
		heroSource = ipCNCand.source
		heroCands = append(heroCands, *ipCNCand)
	case ipipCand != nil:
		heroIP = ipipCand.ip
		heroSource = ipipCand.source
		heroCands = append(heroCands, *ipipCand)
	case traceCand != nil:
		heroIP = traceCand.ip
		heroSource = traceCand.source
	}
	if ipipCand != nil && ipipCand.ip == heroIP && heroSource != "ipip.net" {
		heroCands = append(heroCands, *ipipCand)
	}

	// Collect unique IPs for Phase 4 GeoIP + IPRisk enrichment.
	type ipEnrichment struct {
		geo  *geoData
		risk *IPRisk
	}
	uniqueOrder := make([]string, 0, 6)
	seenIP := map[string]bool{}
	addUniqueIP := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || seenIP[ip] {
			return
		}
		seenIP[ip] = true
		uniqueOrder = append(uniqueOrder, ip)
	}
	addUniqueIP(heroIP)
	for _, sp := range rep.Splits {
		if sp.Status == "ok" && sp.IP != "" {
			addUniqueIP(sp.IP)
		}
	}

	enriched := make(map[string]ipEnrichment, len(uniqueOrder))
	var enrichMu sync.Mutex
	var ewg sync.WaitGroup
	for _, ip := range uniqueOrder {
		ewg.Add(1)
		go func(targetIP string) {
			defer ewg.Done()
			var gd *geoData
			var rk *IPRisk
			var subWG sync.WaitGroup
			subWG.Add(1)
			go func() {
				defer subWG.Done()
				gd = lookupGeo(ctx, client, ep, targetIP)
			}()
			if cfg.IncludeRisk {
				subWG.Add(1)
				go func() {
					defer subWG.Done()
					rk = lookupRisk(ctx, client, ep, targetIP)
				}()
			}
			subWG.Wait()
			enrichMu.Lock()
			enriched[targetIP] = ipEnrichment{geo: gd, risk: rk}
			enrichMu.Unlock()
		}(ip)
	}
	ewg.Wait()

	// Phase 4a: Finalize Hero IP.
	if heroIP != "" {
		en := enriched[heroIP]
		geoZH, src := pickValidatedGeo(en.geo, heroCands)
		if src == "" {
			src = heroSource
		}
		propZH, propEN := formatProperty(en.risk)
		rep.Hero = HeroIP{
			IP:         heroIP,
			Source:     src,
			GeoZH:      geoZH,
			GeoEN:      formatGeoEN(en.geo),
			PropertyZH: propZH,
			PropertyEN: propEN,
			Risk:       en.risk,
		}
		if en.geo != nil {
			rep.Hero.CountryCode = en.geo.CountryCode
			rep.Hero.Country = en.geo.Country
			rep.Hero.Region = en.geo.Region
			rep.Hero.City = en.geo.City
			rep.Hero.ISP = en.geo.ISP
		} else if en.risk != nil {
			rep.Hero.CountryCode = strings.ToLower(en.risk.CountryCode)
			rep.Hero.Country = en.risk.Country
			rep.Hero.Region = en.risk.Region
			rep.Hero.City = en.risk.City
			rep.Hero.ISP = en.risk.ASOrganization
			if rep.Hero.GeoEN == "" {
				rep.Hero.GeoEN = joinUniqueParts(en.risk.Country, en.risk.Region, en.risk.City, en.risk.ASOrganization)
			}
			if rep.Hero.GeoZH == "" {
				rep.Hero.GeoZH = buildZHGeoFromParts(rep.Hero.CountryCode, en.risk.Country, en.risk.Region, en.risk.City, en.risk.ASOrganization)
			}
		}
		if rep.Hero.GeoEN == "" {
			rep.Hero.GeoEN = rep.Hero.GeoZH
		}
	}

	// Phase 4b: Enrich Splits & build deduplicated Exits list in discovery order.
	exitOrder := make([]string, 0, len(uniqueOrder))
	exitSites := map[string][]string{}
	exitCC := map[string]string{}
	for i := range rep.Splits {
		sp := &rep.Splits[i]
		if sp.Status != "ok" || sp.IP == "" {
			continue
		}
		if _, exists := exitSites[sp.IP]; !exists {
			exitOrder = append(exitOrder, sp.IP)
		}
		exitSites[sp.IP] = append(exitSites[sp.IP], sp.Name)
		if exitCC[sp.IP] == "" && sp.CountryCode != "" {
			exitCC[sp.IP] = sp.CountryCode
		}
		en := enriched[sp.IP]
		if en.geo != nil {
			if en.geo.CountryCode != "" {
				sp.CountryCode = en.geo.CountryCode
				if exitCC[sp.IP] == "" {
					exitCC[sp.IP] = en.geo.CountryCode
				}
			}
			sp.GeoEN = formatGeoEN(en.geo)
			sp.GeoZH = buildZHGeoFromParts(sp.CountryCode, en.geo.Country, en.geo.Region, en.geo.City, en.geo.ISP)
		} else if en.risk != nil {
			if sp.CountryCode == "" {
				sp.CountryCode = strings.ToLower(en.risk.CountryCode)
			}
			sp.GeoEN = joinUniqueParts(en.risk.Country, en.risk.Region, en.risk.City, en.risk.ASOrganization)
			sp.GeoZH = buildZHGeoFromParts(sp.CountryCode, en.risk.Country, en.risk.Region, en.risk.City, en.risk.ASOrganization)
		}
		if sp.GeoZH == "" {
			sp.GeoZH = sp.GeoEN
		}
		if sp.GeoEN == "" {
			sp.GeoEN = sp.GeoZH
		}
	}

	rep.Exits = make([]ExitIP, 0, len(exitOrder))
	for _, ip := range exitOrder {
		en := enriched[ip]
		sites := exitSites[ip]
		cc := exitCC[ip]
		propZH, propEN := formatProperty(en.risk)
		ex := ExitIP{
			IP:          ip,
			CountryCode: cc,
			PropertyZH:  propZH,
			PropertyEN:  propEN,
			Count:       len(sites),
			Sites:       sites,
			Risk:        en.risk,
		}
		if en.geo != nil {
			if en.geo.CountryCode != "" {
				ex.CountryCode = en.geo.CountryCode
			}
			ex.Country = en.geo.Country
			ex.Region = en.geo.Region
			ex.City = en.geo.City
			ex.ISP = en.geo.ISP
			ex.GeoEN = formatGeoEN(en.geo)
			ex.GeoZH = buildZHGeoFromParts(ex.CountryCode, en.geo.Country, en.geo.Region, en.geo.City, en.geo.ISP)
		} else if en.risk != nil {
			if ex.CountryCode == "" {
				ex.CountryCode = strings.ToLower(en.risk.CountryCode)
			}
			ex.Country = en.risk.Country
			ex.Region = en.risk.Region
			ex.City = en.risk.City
			ex.ISP = en.risk.ASOrganization
			ex.GeoEN = joinUniqueParts(en.risk.Country, en.risk.Region, en.risk.City, en.risk.ASOrganization)
			ex.GeoZH = buildZHGeoFromParts(ex.CountryCode, en.risk.Country, en.risk.Region, en.risk.City, en.risk.ASOrganization)
		}
		if ip == heroIP && rep.Hero.GeoZH != "" && ex.GeoZH == "" {
			ex.GeoZH = rep.Hero.GeoZH
		}
		if ex.GeoEN == "" {
			ex.GeoEN = ex.GeoZH
		}
		if ex.GeoZH == "" {
			ex.GeoZH = ex.GeoEN
		}
		rep.Exits = append(rep.Exits, ex)
	}

	if rep.Hero.IP == "" && len(rep.Exits) == 0 {
		allFailed := true
		for _, pr := range rep.Probes {
			if pr.Success > 0 {
				allFailed = false
				break
			}
		}
		if allFailed {
			rep.Error = "network probe failed"
		}
	}
	return rep
}

// ---------------------------------------------------------------------------
// Hero IP & Geo validation
// ---------------------------------------------------------------------------

type geoCandidate struct {
	ip     string
	geo    string
	source string
}

type geoData struct {
	Country     string
	Region      string
	City        string
	ISP         string
	CountryCode string
}

func fetchTextBody(ctx context.Context, client *http.Client, targetURL string, timeout time.Duration) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func firstValidIPv4(s string) string {
	for _, m := range ipv4Re.FindAllString(s, -1) {
		if ip := net.ParseIP(m); ip != nil && ip.To4() != nil {
			return m
		}
	}
	return ""
}

func fetchIP138(ctx context.Context, client *http.Client, targetURL string) *geoCandidate {
	body, err := fetchTextBody(ctx, client, targetURL, 5*time.Second)
	if err != nil {
		return nil
	}
	ip := firstValidIPv4(body)
	if ip == "" {
		return nil
	}
	geo := ""
	if m := ip138Geo.FindStringSubmatch(body); len(m) > 1 {
		geo = strings.TrimSpace(m[1])
	}
	return &geoCandidate{ip: ip, geo: geo, source: "iP138.com"}
}

func fetchIPCN(ctx context.Context, client *http.Client, targetURL string) *geoCandidate {
	body, err := fetchTextBody(ctx, client, targetURL, 5*time.Second)
	if err != nil {
		return nil
	}
	ip := firstValidIPv4(body)
	if ip == "" {
		return nil
	}
	geo := ""
	if m := ipcnGeo.FindStringSubmatch(body); len(m) > 1 {
		geo = strings.TrimSpace(m[1])
	}
	return &geoCandidate{ip: ip, geo: geo, source: "IP.cn"}
}

func fetchIPIP(ctx context.Context, client *http.Client, targetURL string) *geoCandidate {
	body, err := fetchTextBody(ctx, client, targetURL, 5*time.Second)
	if err != nil {
		return nil
	}
	var parsed struct {
		Ret  string `json:"ret"`
		Data struct {
			IP       string   `json:"ip"`
			Location []string `json:"location"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &parsed) != nil || parsed.Ret != "ok" {
		return nil
	}
	ip := firstValidIPv4(parsed.Data.IP)
	if ip == "" {
		return nil
	}
	var parts []string
	for _, part := range parsed.Data.Location {
		if t := strings.TrimSpace(part); t != "" {
			parts = append(parts, t)
		}
	}
	return &geoCandidate{ip: ip, geo: strings.Join(parts, " "), source: "ipip.net"}
}

var ccToCNPrefix = map[string][]string{
	"cn": {"中国"}, "hk": {"香港", "中国香港"}, "mo": {"澳门", "中国澳门"}, "tw": {"台湾", "中国台湾"},
	"jp": {"日本"}, "kr": {"韩国"}, "sg": {"新加坡"}, "my": {"马来西亚"},
	"th": {"泰国"}, "vn": {"越南"}, "id": {"印度尼西亚", "印尼"},
	"ph": {"菲律宾"}, "in": {"印度"}, "pk": {"巴基斯坦"}, "bd": {"孟加拉"},
	"ir": {"伊朗"}, "il": {"以色列"}, "ae": {"阿联酋", "阿拉伯联合酋长国"},
	"sa": {"沙特"}, "tr": {"土耳其"},
	"us": {"美国"}, "ca": {"加拿大"}, "mx": {"墨西哥"},
	"br": {"巴西"}, "ar": {"阿根廷"}, "cl": {"智利"},
	"gb": {"英国"}, "ie": {"爱尔兰"}, "fr": {"法国"}, "de": {"德国"},
	"it": {"意大利"}, "es": {"西班牙"}, "pt": {"葡萄牙"}, "nl": {"荷兰"},
	"be": {"比利时"}, "ch": {"瑞士"}, "at": {"奥地利"},
	"se": {"瑞典"}, "no": {"挪威"}, "dk": {"丹麦"}, "fi": {"芬兰"},
	"pl": {"波兰"}, "cz": {"捷克"}, "ru": {"俄罗斯", "俄国"},
	"ua": {"乌克兰"}, "au": {"澳大利亚", "澳洲"}, "nz": {"新西兰"},
	"za": {"南非"}, "eg": {"埃及"}, "ng": {"尼日利亚"}, "ke": {"肯尼亚"},
}

var ccToCNName = map[string]string{
	"cn": "中国", "hk": "中国香港", "mo": "中国澳门", "tw": "中国台湾",
	"jp": "日本", "kr": "韩国", "sg": "新加坡", "my": "马来西亚",
	"th": "泰国", "vn": "越南", "id": "印度尼西亚", "ph": "菲律宾",
	"in": "印度", "pk": "巴基斯坦", "bd": "孟加拉国",
	"ir": "伊朗", "il": "以色列", "ae": "阿联酋", "sa": "沙特",
	"tr": "土耳其", "us": "美国", "ca": "加拿大", "mx": "墨西哥",
	"br": "巴西", "ar": "阿根廷", "cl": "智利",
	"gb": "英国", "ie": "爱尔兰", "fr": "法国", "de": "德国",
	"it": "意大利", "es": "西班牙", "pt": "葡萄牙", "nl": "荷兰",
	"be": "比利时", "ch": "瑞士", "at": "奥地利",
	"se": "瑞典", "no": "挪威", "dk": "丹麦", "fi": "芬兰",
	"pl": "波兰", "cz": "捷克", "ru": "俄罗斯", "ua": "乌克兰",
	"au": "澳大利亚", "nz": "新西兰", "za": "南非", "eg": "埃及",
	"ng": "尼日利亚", "ke": "肯尼亚",
}

func matchesCCPrefix(prefixes []string, text string) bool {
	if len(prefixes) == 0 || text == "" {
		return false
	}
	for _, p := range prefixes {
		if !strings.HasPrefix(text, p) {
			continue
		}
		rest := strings.TrimPrefix(text, p)
		if rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "　") {
			return true
		}
	}
	return false
}

func pickValidatedGeo(gd *geoData, cands []geoCandidate) (string, string) {
	if gd == nil || gd.CountryCode == "" {
		best := geoCandidate{}
		for _, c := range cands {
			if len(c.geo) > len(best.geo) {
				best = c
			}
		}
		if best.geo != "" {
			return best.geo, best.source
		}
		if len(cands) > 0 {
			return cands[0].geo, cands[0].source
		}
		return "", ""
	}
	cc := strings.ToLower(gd.CountryCode)
	prefixes := ccToCNPrefix[cc]
	var valid []geoCandidate
	for _, c := range cands {
		if c.geo != "" && matchesCCPrefix(prefixes, c.geo) {
			valid = append(valid, c)
		}
	}
	if len(valid) > 0 {
		best := valid[0]
		for _, c := range valid[1:] {
			if len(c.geo) > len(best.geo) {
				best = c
			}
		}
		src := best.source
		if len(valid) > 1 {
			src = "iP138.com / IP.cn"
		}
		return best.geo, src
	}
	return buildZHGeoFromParts(cc, gd.Country, gd.Region, gd.City, gd.ISP), "GeoIP"
}

func buildZHGeoFromParts(cc, country, region, city, isp string) string {
	cnName := ccToCNName[strings.ToLower(cc)]
	if cnName == "" {
		cnName = country
	}
	if cnName == "" {
		cnName = "未知"
	}
	parts := []string{cnName}
	if region != "" && region != city {
		parts = append(parts, region)
	}
	if city != "" {
		parts = append(parts, city)
	}
	if isp != "" {
		parts = append(parts, isp)
	}
	return strings.Join(parts, " ")
}

func formatGeoEN(gd *geoData) string {
	if gd == nil {
		return ""
	}
	return joinUniqueParts(gd.Country, gd.Region, gd.City, gd.ISP)
}

func joinUniqueParts(country, region, city, isp string) string {
	var parts []string
	if country != "" {
		parts = append(parts, country)
	}
	if region != "" && region != city && region != country {
		parts = append(parts, region)
	}
	if city != "" {
		parts = append(parts, city)
	}
	if isp != "" {
		parts = append(parts, isp)
	}
	return strings.Join(parts, " ")
}

func lookupGeo(ctx context.Context, client *http.Client, ep Endpoints, ip string) *geoData {
	if ip == "" {
		return nil
	}
	esc := url.PathEscape(ip)

	// Primary: ip.net.coffee /api/geoip/{ip}
	if ep.GeoIPBase != "" {
		if body, err := fetchTextBody(ctx, client, ep.GeoIPBase+esc, 4*time.Second); err == nil {
			var d struct {
				Country     string `json:"country"`
				Region      string `json:"region"`
				City        string `json:"city"`
				ISP         string `json:"isp"`
				CountryCode string `json:"country_code"`
			}
			if json.Unmarshal([]byte(body), &d) == nil && (d.Country != "" || d.CountryCode != "") {
				return &geoData{
					Country:     d.Country,
					Region:      d.Region,
					City:        d.City,
					ISP:         d.ISP,
					CountryCode: strings.ToLower(d.CountryCode),
				}
			}
		}
	}

	// Fallback 1: api.ip.sb
	if ep.GeoFallback1 != "" {
		if body, err := fetchTextBody(ctx, client, ep.GeoFallback1+esc, 4*time.Second); err == nil {
			var d struct {
				Country      string `json:"country"`
				Region       string `json:"region"`
				City         string `json:"city"`
				ISP          string `json:"isp"`
				Organization string `json:"organization"`
				CountryCode  string `json:"country_code"`
			}
			if json.Unmarshal([]byte(body), &d) == nil && d.Country != "" {
				isp := d.ISP
				if isp == "" {
					isp = d.Organization
				}
				return &geoData{
					Country:     d.Country,
					Region:      d.Region,
					City:        d.City,
					ISP:         isp,
					CountryCode: strings.ToLower(d.CountryCode),
				}
			}
		}
	}

	// Fallback 2: ipwho.is
	if ep.GeoFallback2 != "" {
		if body, err := fetchTextBody(ctx, client, ep.GeoFallback2+esc, 4*time.Second); err == nil {
			var d struct {
				Success     bool   `json:"success"`
				Country     string `json:"country"`
				Region      string `json:"region"`
				City        string `json:"city"`
				CountryCode string `json:"country_code"`
				Connection  struct {
					ISP string `json:"isp"`
				} `json:"connection"`
			}
			if json.Unmarshal([]byte(body), &d) == nil && d.Success {
				return &geoData{
					Country:     d.Country,
					Region:      d.Region,
					City:        d.City,
					ISP:         d.Connection.ISP,
					CountryCode: strings.ToLower(d.CountryCode),
				}
			}
		}
	}
	return nil
}

func lookupRisk(ctx context.Context, client *http.Client, ep Endpoints, ip string) *IPRisk {
	if ip == "" || ep.IPRiskBase == "" {
		return nil
	}
	body, err := fetchTextBody(ctx, client, ep.IPRiskBase+url.PathEscape(ip), 4*time.Second)
	if err != nil {
		return nil
	}
	var rk IPRisk
	if json.Unmarshal([]byte(body), &rk) != nil {
		return nil
	}
	if rk.IP == "" {
		rk.IP = ip
	}
	return &rk
}

func formatProperty(rk *IPRisk) (zh, en string) {
	if rk == nil {
		return "", ""
	}
	typeMap := map[string]string{
		"isp":       "ISP",
		"hosting":   "Hosting",
		"business":  "Business",
		"education": "Education",
	}
	t := strings.ToLower(strings.TrimSpace(rk.CompanyType))
	tStr := typeMap[t]
	if tStr == "" {
		tStr = rk.CompanyType
	}
	suffix := ""
	if tStr != "" {
		suffix = " (" + tStr + ")"
	}
	isRes := rk.IsResidential != nil && *rk.IsResidential
	switch {
	case isRes && t == "business":
		return "商业宽带 (Business)", "Business Broadband (Business)"
	case isRes:
		return "家庭宽带" + suffix, "Residential" + suffix
	case rk.IsDatacenter && t == "business":
		return "商业专线 (Business)", "Business Dedicated (Business)"
	case rk.IsDatacenter:
		return "机房 IP" + suffix, "Datacenter" + suffix
	case t == "education":
		return "专用 (Education)", "Education"
	case tStr != "":
		return "未知 (" + tStr + ")", "Unknown (" + tStr + ")"
	default:
		return "未知", "Unknown"
	}
}

// ---------------------------------------------------------------------------
// 12-round HTTP keep-alive connectivity probes
// ---------------------------------------------------------------------------

func noRedirectClient(base *http.Client) *http.Client {
	cp := *base
	cp.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &cp
}

func singlePing(ctx context.Context, client *http.Client, targetURL string, timeout time.Duration) int {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return -1
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Cache-Control", "no-store")
	t0 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return -1
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	ms := int(time.Since(t0).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return ms
}

func runProbeTarget(ctx context.Context, client *http.Client, target ProbeTarget, rounds int) ProbeResult {
	res := ProbeResult{
		Name:     target.Name,
		EnName:   target.EnName,
		Icon:     target.Icon,
		Flag:     target.Flag,
		Tag:      target.Tag,
		URL:      target.URL,
		MedianMs: -1,
		MinMs:    -1,
		MaxMs:    -1,
		Total:    rounds,
		Dots:     make([]int, rounds),
	}
	for i := range res.Dots {
		res.Dots[i] = -1
	}

	targetCtx, cancel := context.WithTimeout(ctx, 4200*time.Millisecond)
	defer cancel()

	// Hidden warmup request so TCP+TLS connection pools are established before measured rounds.
	_ = singlePing(targetCtx, client, target.URL, 1800*time.Millisecond)

	const (
		perRoundTimeout = 1500 * time.Millisecond
		minSpacing      = 45 * time.Millisecond
	)

	valid := make([]int, 0, rounds)
	consecutiveFails := 0
	for r := 0; r < rounds; r++ {
		if targetCtx.Err() != nil {
			break
		}
		roundStart := time.Now()
		ms := singlePing(targetCtx, client, target.URL, perRoundTimeout)
		res.Dots[r] = ms
		if ms >= 0 {
			valid = append(valid, ms)
			consecutiveFails = 0
		} else {
			consecutiveFails++
			if consecutiveFails >= 3 {
				break
			}
		}
		if r < rounds-1 {
			elapsed := time.Since(roundStart)
			if elapsed < minSpacing {
				timer := time.NewTimer(minSpacing - elapsed)
				select {
				case <-targetCtx.Done():
					timer.Stop()
					return finalizeProbe(res, valid)
				case <-timer.C:
				}
			}
		}
	}
	return finalizeProbe(res, valid)
}

func finalizeProbe(res ProbeResult, valid []int) ProbeResult {
	res.Success = len(valid)
	if len(valid) == 0 {
		return res
	}
	sorted := append([]int(nil), valid...)
	sort.Ints(sorted)
	res.MinMs = sorted[0]
	res.MaxMs = sorted[len(sorted)-1]
	res.MedianMs = sorted[len(sorted)/2]
	return res
}

// ---------------------------------------------------------------------------
// Website split-routing tests
// ---------------------------------------------------------------------------

func runSplitTests(ctx context.Context, client *http.Client, ep Endpoints, splits []SplitTarget, out []SplitResult) {
	const workers = 12
	jobs := make(chan int, len(splits))
	for i := range splits {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < min(workers, len(splits)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				out[idx] = runOneSplit(ctx, client, ep, splits[idx])
			}
		}()
	}
	wg.Wait()

	// Retry failed entries once if context still has time.
	var failed []int
	for i := range out {
		if out[i].Status == "error" {
			failed = append(failed, i)
		}
	}
	if len(failed) == 0 || ctx.Err() != nil {
		return
	}
	timer := time.NewTimer(250 * time.Millisecond)
	select {
	case <-ctx.Done():
		timer.Stop()
		return
	case <-timer.C:
	}

	retryJobs := make(chan int, len(failed))
	for _, idx := range failed {
		retryJobs <- idx
	}
	close(retryJobs)
	for w := 0; w < min(workers, len(failed)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range retryJobs {
				out[idx] = runOneSplit(ctx, client, ep, splits[idx])
			}
		}()
	}
	wg.Wait()
}

func runOneSplit(ctx context.Context, client *http.Client, ep Endpoints, target SplitTarget) SplitResult {
	res := SplitResult{
		Name:      target.Name,
		Icon:      target.Icon,
		Type:      target.Type,
		Category:  target.Category,
		LatencyMs: -1,
		Status:    "error",
	}
	t0 := time.Now()
	var (
		ip, cc string
		err    error
	)
	switch target.Method {
	case "netease":
		u := target.URL
		if ep.NeteaseURL != defaultNeteaseURL || u == "" {
			u = ep.NeteaseURL
		}
		ip, err = detectHeadIP(ctx, client, u, "cdn-user-ip")
	case "bytedance":
		u := target.URL
		if ep.BytedanceURL != defaultBytedanceURL || u == "" {
			u = ep.BytedanceURL
		}
		ip, err = detectHeadIP(ctx, client, u, "x-request-ip", "x-response-cinfo")
	default: // cftrace
		u := fmt.Sprintf(ep.CFTraceHostFmt, target.Domain)
		var tr cfTraceResult
		tr, err = fetchCFTraceURL(ctx, client, u)
		ip, cc = tr.ip, tr.loc
	}
	ms := int(time.Since(t0).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	res.LatencyMs = ms
	if err != nil || ip == "" {
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Error = "no IP returned"
		}
		return res
	}
	res.Status = "ok"
	res.IP = ip
	res.CountryCode = cc
	return res
}

type cfTraceResult struct {
	ip  string
	loc string
}

func fetchCFTraceURL(ctx context.Context, client *http.Client, targetURL string) (cfTraceResult, error) {
	body, err := fetchTextBody(ctx, client, targetURL, 4*time.Second)
	if err != nil {
		return cfTraceResult{}, err
	}
	var out cfTraceResult
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "ip":
			out.ip = strings.TrimSpace(v)
		case "loc":
			out.loc = strings.ToLower(strings.TrimSpace(v))
		}
	}
	if out.ip == "" {
		return cfTraceResult{}, fmt.Errorf("missing ip in trace")
	}
	return out, nil
}

func detectHeadIP(ctx context.Context, client *http.Client, targetURL string, headers ...string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Cache-Control", "no-store")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	for _, h := range headers {
		if v := strings.TrimSpace(resp.Header.Get(h)); v != "" {
			if ip := firstValidIPv4(v); ip != "" {
				return ip, nil
			}
			if parsed := net.ParseIP(v); parsed != nil {
				return v, nil
			}
		}
	}
	return "", fmt.Errorf("header missing IP")
}
