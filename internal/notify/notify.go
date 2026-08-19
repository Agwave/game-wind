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

// BuildMessages 把 header + body 按行拆成多条 ≤MaxBytes 的消息，footer 追加到最后一条。
// 完整报告因此能分段完整推送，不丢内容；单条过长时在行边界换段。
func BuildMessages(header, body, footer string) ([]string, error) {
	content := header + "\n" + body
	if content == "" {
		content = header
	}
	var msgs []string
	var buf []string
	size := 0
	flush := func() {
		if len(buf) == 0 {
			return
		}
		msgs = append(msgs, strings.Join(buf, "\n"))
		buf = buf[:0]
		size = 0
	}
	for _, l := range strings.Split(content, "\n") {
		cost := len(l) + 1 // 含换行
		if size > 0 && size+cost > MaxBytes {
			flush()
		}
		buf = append(buf, l)
		size += cost
	}
	flush()
	if len(msgs) == 0 {
		msgs = []string{""}
	}
	msgs[len(msgs)-1] += "\n" + footer
	return msgs, nil
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
