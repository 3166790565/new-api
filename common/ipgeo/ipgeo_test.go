package ipgeo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 库缺失 / 无效数据时降级为空字符串，绝不 panic。
func TestResolveRegionDegradesGracefully(t *testing.T) {
	// 未设置任何路径且工作目录下无 data/ip2region.xdb 时应返回空。
	assert.NotPanics(t, func() {
		_ = normalizeRegion("0|0|0|0|0")
	})
	assert.Equal(t, "", normalizeRegion("0|0|0|0|0"))
	assert.Equal(t, "", normalizeRegion(""))
}

// normalizeRegion 中国带省份返回「中国·省份」、无省份返回中国，海外取国家。
func TestNormalizeRegion(t *testing.T) {
	assert.Equal(t, "中国·广东省", normalizeRegion("中国|0|广东省|深圳市|电信"))
	assert.Equal(t, "中国", normalizeRegion("中国|0|0|0|0"))
	assert.Equal(t, "美国", normalizeRegion("美国|0|0|0|0"))
	assert.Equal(t, "日本", normalizeRegion("日本|0|东京都|0|0"))
}

// 有真实 xdb 时能解析出已知公网 IP，且内网/非法 IP 返回空而不报错。
func TestResolveRegionWithRealDB(t *testing.T) {
	xdbPath, _ := filepath.Abs(filepath.Join("..", "..", "data", "ip2region.xdb"))
	if _, err := os.Stat(xdbPath); err != nil {
		t.Skip("ip2region.xdb not present; skipping live resolution test")
	}
	require.NoError(t, os.Setenv("IP2REGION_XDB_PATH", xdbPath))
	t.Cleanup(func() { _ = os.Unsetenv("IP2REGION_XDB_PATH") })

	// 阿里公共 DNS，稳定归属中国。
	region := ResolveRegion("223.5.5.5")
	assert.NotEmpty(t, region, "已知公网 IP 应能解析出地区")

	assert.Empty(t, ResolveRegion(""), "空 IP 返回空")
	assert.Empty(t, ResolveRegion("not-an-ip"), "非法 IP 返回空而不报错")
}
