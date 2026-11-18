package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The remote providers call the balance endpoint their service documents,
// with a key the user pasted in; they read what is left and send nothing else.

func getJSON(ctx context.Context, o Options, url, bearer string, out any) error {
	if o.BaseURL != "" {
		// A test or a proxy: keep the path, change the origin.
		i := strings.Index(url[8:], "/")
		url = strings.TrimRight(o.BaseURL, "/") + url[8+i:]
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	client := o.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return errors.New("密钥不对或没有权限（HTTP " + strconv.Itoa(resp.StatusCode) + "）")
	case resp.StatusCode/100 != 2:
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(strings.ReplaceAll(x, ",", ""), 64)
		return f
	}
	return 0
}

// balanceReport turns a balance into a report: what is left, and with a
// budget (what was put in) the share used.
func balanceReport(unit string, balance float64, o Options) Report {
	r := Report{Unit: unit, Balance: balance, HasBalance: true, Windows: map[string]float64{}}
	if o.Budget > 0 {
		r.Limit = o.Budget
		r.Used = o.Budget - balance
	}
	return r
}

type deepseek struct{}

func (deepseek) ID() string     { return "deepseek-balance" }
func (deepseek) Name() string   { return "DeepSeek 余额" }
func (deepseek) NeedsKey() bool { return true }
func (deepseek) Fetch(ctx context.Context, o Options) (Report, error) {
	var out struct {
		Infos []struct {
			Currency string `json:"currency"`
			Total    any    `json:"total_balance"`
		} `json:"balance_infos"`
	}
	if err := getJSON(ctx, o, "https://api.deepseek.com/user/balance", o.Key, &out); err != nil {
		return Report{}, err
	}
	if len(out.Infos) == 0 {
		return Report{}, errors.New("响应里没有余额")
	}
	return balanceReport(out.Infos[0].Currency, num(out.Infos[0].Total), o), nil
}

type openrouter struct{}

func (openrouter) ID() string     { return "openrouter-credits" }
func (openrouter) Name() string   { return "OpenRouter 额度" }
func (openrouter) NeedsKey() bool { return true }
func (openrouter) Fetch(ctx context.Context, o Options) (Report, error) {
	var out struct {
		Data struct {
			Credits float64 `json:"total_credits"`
			Usage   float64 `json:"total_usage"`
		} `json:"data"`
	}
	if err := getJSON(ctx, o, "https://openrouter.ai/api/v1/credits", o.Key, &out); err != nil {
		return Report{}, err
	}
	r := balanceReport("USD", out.Data.Credits-out.Data.Usage, o)
	// OpenRouter reports its own limit: what has been bought.
	r.Limit, r.Used = out.Data.Credits, out.Data.Usage
	return r, nil
}

type moonshot struct{}

func (moonshot) ID() string     { return "moonshot-balance" }
func (moonshot) Name() string   { return "Moonshot 余额" }
func (moonshot) NeedsKey() bool { return true }
func (moonshot) Fetch(ctx context.Context, o Options) (Report, error) {
	var out struct {
		Data struct {
			Available any `json:"available_balance"`
		} `json:"data"`
	}
	if err := getJSON(ctx, o, "https://api.moonshot.cn/v1/users/me/balance", o.Key, &out); err != nil {
		return Report{}, err
	}
	return balanceReport("CNY", num(out.Data.Available), o), nil
}

type siliconflow struct{}

func (siliconflow) ID() string     { return "siliconflow-balance" }
func (siliconflow) Name() string   { return "SiliconFlow 余额" }
func (siliconflow) NeedsKey() bool { return true }
func (siliconflow) Fetch(ctx context.Context, o Options) (Report, error) {
	var out struct {
		Data struct {
			Total any `json:"totalBalance"`
		} `json:"data"`
	}
	if err := getJSON(ctx, o, "https://api.siliconflow.cn/v1/user/info", o.Key, &out); err != nil {
		return Report{}, err
	}
	return balanceReport("CNY", num(out.Data.Total), o), nil
}
