package notify

import (
	"strings"
	"testing"
)

func TestBuildMessagesShort(t *testing.T) {
	msgs, err := BuildMessages("# 标题", "正文", "footer")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("短内容应单条, 实际 %d 条", len(msgs))
	}
	if !strings.Contains(msgs[0], "footer") {
		t.Errorf("footer 应保留: %s", msgs[0])
	}
}

// TestBuildMessagesSplit 长正文应按行边界拆成多条，且 footer 追加到最后一条。
func TestBuildMessagesSplit(t *testing.T) {
	line := strings.Repeat("x", 100)
	body := strings.Repeat(line+"\n", 100) // 约 10100 字节，远超 MaxBytes
	msgs, err := BuildMessages("H", body, "FOOTER")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) < 2 {
		t.Fatalf("应拆成多条, 实际 %d 条", len(msgs))
	}
	if !strings.Contains(msgs[len(msgs)-1], "FOOTER") {
		t.Errorf("footer 应追加到最后一条")
	}
	for _, m := range msgs {
		if len(m) > MaxBytes+10 {
			t.Errorf("单条超过 MaxBytes: %d 字节", len(m))
		}
	}
}
