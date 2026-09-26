package ipgeo

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// 离线 IP 地区解析：基于 ip2region 的 xdb 数据文件，全量载入内存，不外发用户 IP。
// 数据文件缺失或解析失败时全程降级为空字符串，绝不阻断主流程。

var (
	initOnce    sync.Once
	loaded      bool
	contentBuff []byte
	dbVersion   *xdb.Version
)

// Init 载入 xdb 数据文件（幂等，可在启动时调用一次）。
func Init() {
	initOnce.Do(load)
}

func resolvePath() string {
	if p := strings.TrimSpace(os.Getenv("IP2REGION_XDB_PATH")); p != "" {
		if fileExists(p) {
			return p
		}
		common.SysLog("IP2REGION_XDB_PATH points to a missing file: " + p)
	}
	candidates := []string{filepath.Join("data", "ip2region.xdb")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "data", "ip2region.xdb"))
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func load() {
	path := resolvePath()
	if path == "" {
		common.SysLog("ip2region xdb not found; region distribution will be marked unknown")
		return
	}
	buff, err := xdb.LoadContentFromFile(path)
	if err != nil {
		common.SysLog("failed to load ip2region xdb: " + err.Error())
		return
	}
	header, err := xdb.LoadHeaderFromBuff(buff)
	if err != nil {
		common.SysLog("failed to parse ip2region xdb header: " + err.Error())
		return
	}
	version, err := xdb.VersionFromHeader(header)
	if err != nil {
		common.SysLog("failed to detect ip2region xdb version: " + err.Error())
		return
	}
	contentBuff = buff
	dbVersion = version
	loaded = true
	common.SysLog("ip2region xdb loaded from " + path + " (" + version.Name + ")")
}

// ResolveRegion 把 IP 解析为可读地区（中国返回「中国·省份」，其余返回国家）。
// 未加载数据、无法解析或 IP 版本不匹配时返回空字符串。
func ResolveRegion(ip string) string {
	Init()
	if !loaded {
		return ""
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	searcher, err := xdb.NewWithBuffer(dbVersion, contentBuff)
	if err != nil {
		return ""
	}
	// contentBuff 为共享只读缓冲，每次新建 searcher 以规避其非线程安全的可变状态。
	raw, err := searcher.Search(ip)
	if err != nil {
		return ""
	}
	return normalizeRegion(raw)
}

// normalizeRegion 解析 ip2region 的 "国家|区域|省份|城市|ISP" 结构，取有意义的地区名。
// 中国带省份时返回「中国·省份」（如 中国·广东省），无省份时返回「中国」；海外返回国家名。
func normalizeRegion(raw string) string {
	parts := strings.Split(raw, "|")
	field := func(i int) string {
		if i < len(parts) {
			s := strings.TrimSpace(parts[i])
			if s != "" && s != "0" {
				return s
			}
		}
		return ""
	}
	country := field(0)
	province := field(2)
	if country == "中国" || strings.EqualFold(country, "China") {
		if province != "" {
			return "中国·" + province
		}
		return "中国"
	}
	if country != "" {
		return country
	}
	return province
}
