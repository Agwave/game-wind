package mapping

import (
	"os"
	"path/filepath"
	"testing"
)

const testYAML = `
companies:
  - company: 腾讯控股
    code: "0700.HK"
    market: 港股
    games:
      - ids: [id16]
        names: [穿越火线-枪战王者]
      - ids: [id17, id170]
        names: [金铲铲之战]
        note: 双地区不同 id
      - names: [元梦之星]
      - contains: [Delta Force]
  - company: 网易
    code: "9999.HK"
    market: 港股
    games:
      - ids: [id8]
        names: [梦幻西游]
`

func loadTable(t *testing.T) *Table {
	t.Helper()
	p := filepath.Join(t.TempDir(), "games.yaml")
	if err := os.WriteFile(p, []byte(testYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	tab, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return tab
}

func TestMatchByID(t *testing.T) {
	tab := loadTable(t)
	m := tab.Match("id16", "标题被本地化改了")
	if m == nil || m.Company != "腾讯控股" {
		t.Errorf("id 匹配失败: %+v", m)
	}
	// 多 id 条目
	if m := tab.Match("id170", "金铲铲之战"); m == nil || m.Company != "腾讯控股" {
		t.Errorf("第二个 id 匹配失败: %+v", m)
	}
}

func TestMatchByNameExact(t *testing.T) {
	tab := loadTable(t)
	m := tab.Match("999999", "元梦之星")
	if m == nil || m.Company != "腾讯控股" || m.Game != "元梦之星" {
		t.Errorf("精确名匹配失败: %+v", m)
	}
}

func TestMatchByNameContains(t *testing.T) {
	tab := loadTable(t)
	m := tab.Match("888888", "Delta Force Mobile")
	if m == nil || m.Company != "腾讯控股" {
		t.Errorf("contains 匹配失败: %+v", m)
	}
	// 包含匹配不命中精确名已有条目
	if m := tab.Match("1", "王者荣耀"); m != nil {
		t.Errorf("不应匹配到任何公司: %+v", m)
	}
}

func TestMatchPriorityIDOverName(t *testing.T) {
	tab := loadTable(t)
	// id16 属于腾讯，即使名字写成网易的游戏也不应改判
	m := tab.Match("id16", "梦幻西游")
	if m == nil || m.Company != "腾讯控股" {
		t.Errorf("id 优先级应高于名字: %+v", m)
	}
}

func TestLoadErrors(t *testing.T) {
	p := filepath.Join(t.TempDir(), "games.yaml")
	if err := os.WriteFile(p, []byte("companies:\n  - games: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Error("缺 company 的条目应报错")
	}
}

// TestUnrelatedArtists 无关发行商名单：加载、小写子串匹配、空 artist、不误伤目标公司。
func TestUnrelatedArtists(t *testing.T) {
	yamlContent := `
unrelated_artists:
  - Nintendo
  - miHoYo
  - King
companies:
  - company: 腾讯控股
    code: "0700.HK"
    market: 港股
    games:
      - names: [王者荣耀]
`
	p := filepath.Join(t.TempDir(), "games.yaml")
	if err := os.WriteFile(p, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	tab, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.Companies) != 1 || tab.Companies[0].Company != "腾讯控股" {
		t.Errorf("Companies 应保留 yaml 顺序: %+v", tab.Companies)
	}
	if !tab.IsUnrelated("Nintendo Co., Ltd.") {
		t.Error("小写子串应命中 Nintendo Co., Ltd.")
	}
	if !tab.IsUnrelated("miHoYo Limited") {
		t.Error("应命中 miHoYo Limited")
	}
	if tab.IsUnrelated("") {
		t.Error("空 artist 不应命中")
	}
	if tab.IsUnrelated("Century Games Pte. Ltd.") {
		t.Error("Century（目标公司出海品牌）不应命中")
	}
}
