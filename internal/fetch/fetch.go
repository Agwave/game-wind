// Package fetch 从苹果 iTunes RSS 接口抓取各地区 iOS 游戏榜单。
//
// 数据源（已验证可用，无需鉴权）：
//
//	https://itunes.apple.com/{cc}/rss/{feed}/genre=6014/limit=100/json
//	feed: topgrossingapplications（畅销榜）/ topfreeapplications（免费榜）
//	genre=6014 即「游戏」分类。
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// 榜单类型。
const (
	ChartGrossing = "topgrossingapplications" // 畅销榜
	ChartFree     = "topfreeapplications"     // 免费榜
)

// ChartNames 用于展示的榜单名。
var ChartNames = map[string]string{
	ChartGrossing: "畅销榜",
	ChartFree:     "免费榜",
}

// Entry 榜单中的一款游戏。
type Entry struct {
	Rank   int    `json:"rank"`   // 从 1 开始
	AppID  string `json:"app_id"` // App Store app id（跨地区稳定）
	Name   string `json:"name"`   // 该地区的本地化标题
	Artist string `json:"artist"` // 开发者账号名
}

// Client 苹果 RSS 抓取客户端。
type Client struct {
	http  *http.Client
	limit int
}

// New 创建客户端，limit 为每个榜单抓取的条数（上限 100）。
func New(limit int) *Client {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	return &Client{
		http:  &http.Client{Timeout: 30 * time.Second},
		limit: limit,
	}
}

// 苹果 RSS 的 JSON 结构（entry 只有一条记录时是对象而不是数组，需要兼容）。
type rssFeed struct {
	Feed struct {
		Entry json.RawMessage `json:"entry"`
	} `json:"feed"`
}

type rssEntry struct {
	IMName struct {
		Label string `json:"label"`
	} `json:"im:name"`
	IMArtist *struct {
		Label string `json:"label"`
	} `json:"im:artist"`
	ID struct {
		Label string `json:"label"`
	} `json:"id"`
}

var idURLRe = regexp.MustCompile(`/id(\d+)`)

// FetchChart 抓取指定地区、指定榜单，带重试（2 次，指数退避）。
func (c *Client) FetchChart(ctx context.Context, cc, chart string) ([]Entry, error) {
	url := fmt.Sprintf("https://itunes.apple.com/%s/rss/%s/genre=6014/limit=%d/json", cc, chart, c.limit)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		entries, err := c.fetchOnce(ctx, url)
		if err == nil {
			return entries, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("抓取 %s/%s 失败（已重试 2 次）: %w", cc, chart, lastErr)
}

func (c *Client) fetchOnce(ctx context.Context, url string) ([]Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return Parse(body)
}

// Parse 解析苹果 RSS 的 JSON 响应（entry 只有一条记录时是对象而不是数组，需要兼容）。
func Parse(body []byte) ([]Entry, error) {
	var feed rssFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("解析响应: %w", err)
	}
	if len(feed.Feed.Entry) == 0 {
		return nil, fmt.Errorf("响应为空")
	}
	var list []rssEntry
	if feed.Feed.Entry[0] == '{' { // 单条记录时 entry 是对象
		var one rssEntry
		if err := json.Unmarshal(feed.Feed.Entry, &one); err != nil {
			return nil, err
		}
		list = []rssEntry{one}
	} else {
		if err := json.Unmarshal(feed.Feed.Entry, &list); err != nil {
			return nil, err
		}
	}
	entries := make([]Entry, 0, len(list))
	for i, e := range list {
		appID := ""
		if m := idURLRe.FindStringSubmatch(e.ID.Label); m != nil {
			appID = m[1]
		}
		artist := ""
		if e.IMArtist != nil {
			artist = e.IMArtist.Label
		}
		entries = append(entries, Entry{Rank: i + 1, AppID: appID, Name: e.IMName.Label, Artist: artist})
	}
	return entries, nil
}
