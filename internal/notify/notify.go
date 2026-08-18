// Package notify 通过企业微信群机器人 webhook 推送消息。
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WeCom 企业微信群机器人客户端。
type WeCom struct {
	webhook string
	http    *http.Client
}

// New 创建客户端；webhook 为空时返回 nil（表示不启用推送）。
func New(webhook string) *WeCom {
	if webhook == "" {
		return nil
	}
	return &WeCom{webhook: webhook, http: &http.Client{Timeout: 15 * time.Second}}
}

// MaxBytes 企业微信 markdown 消息体上限（4096 字节），留一点余量。
const MaxBytes = 4000

// SendMarkdown 发送一条 markdown 消息。
func (w *WeCom) SendMarkdown(content string) error {
	if w == nil {
		return nil
	}
	payload := map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": content},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, w.webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("企业微信推送失败: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var r struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		return fmt.Errorf("企业微信返回异常: %s", string(rb))
	}
	if r.ErrCode != 0 {
		return fmt.Errorf("企业微信返回错误 %d: %s", r.ErrCode, r.ErrMsg)
	}
	return nil
}

// BuildMessages 按地区拆分推送内容：每条 = header + 一个地区小节 + footer。
// 小节超过 MaxBytes 时按行截断并注明省略条数（完整报告在 data/reports/ 落盘，不丢失信息）。
func BuildMessages(header string, sections map[string]string, footer string) ([]string, error) {
	var msgs []string
	for _, cc := range sortedKeys(sections) {
		sec := sections[cc]
		if sec == "" {
			continue
		}
		msg := header + "\n" + sec + "\n" + footer
		if len(msg) > MaxBytes {
			msg = header + "\n" + truncateSection(sec) + "\n" + footer
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// truncateSection 按行截断地区小节，保证整体不超过 MaxBytes。
func truncateSection(sec string) string {
	base := MaxBytes - 64 // 给 header/footer 和省略说明预留
	lines := strings.Split(sec, "\n")
	var kept []string
	used := 0
	for _, l := range lines {
		cost := len(l) + 1 // 含换行
		if used+cost > base {
			break
		}
		kept = append(kept, l)
		used += cost
	}
	skipped := len(lines) - len(kept)
	if skipped > 0 {
		kept = append(kept, fmt.Sprintf("…（内容过长，已省略 %d 条，完整报告见 data/reports/）", skipped))
	}
	return strings.Join(kept, "\n")
}

// SendAll 依次发送多条消息。
func (w *WeCom) SendAll(msgs []string) error {
	if w == nil {
		return nil
	}
	for _, m := range msgs {
		if err := w.SendMarkdown(m); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 固定顺序：cn, us, jp, kr
	order := []string{"cn", "us", "jp", "kr"}
	seen := make(map[string]bool, len(keys))
	var out []string
	for _, c := range order {
		if _, ok := m[c]; ok {
			out = append(out, c)
			seen[c] = true
		}
	}
	for _, k := range keys {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}
