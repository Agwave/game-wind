package fetch

import "testing"

func TestParseList(t *testing.T) {
	body := []byte(`{"feed":{"entry":[
		{"im:name":{"label":"王者荣耀"},"im:artist":{"label":"Tencent"},"id":{"label":"https://itunes.apple.com/cn/app/x/id989673964?mt=8"}},
		{"im:name":{"label":"和平精英"},"id":{"label":"https://itunes.apple.com/cn/app/y/id1321803705?mt=8"}}
	]}}`)
	entries, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("期望 2 条, 实际 %d", len(entries))
	}
	if entries[0].Rank != 1 || entries[0].Name != "王者荣耀" || entries[0].AppID != "989673964" || entries[0].Artist != "Tencent" {
		t.Errorf("第 1 条解析错误: %+v", entries[0])
	}
	if entries[1].Rank != 2 || entries[1].AppID != "1321803705" || entries[1].Artist != "" {
		t.Errorf("第 2 条解析错误: %+v", entries[1])
	}
}

func TestParseSingleEntry(t *testing.T) {
	// 榜单只有一条记录时 entry 是对象而不是数组
	body := []byte(`{"feed":{"entry":{"im:name":{"label":"唯一游戏"},"id":{"label":"https://itunes.apple.com/cn/app/x/id123?mt=8"}}}}`)
	entries, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].AppID != "123" {
		t.Errorf("单条对象解析失败: %+v", entries)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse([]byte(`{"feed":{}}`)); err == nil {
		t.Error("空响应应报错")
	}
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Error("非法 JSON 应报错")
	}
}
