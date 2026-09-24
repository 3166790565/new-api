package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetStatFixture(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&StatDailyActive{}))
	require.NoError(t, DB.Where("1 = 1").Delete(&StatDailyActive{}).Error)
	statDedupLock.Lock()
	statDedupCache = make(map[string]struct{})
	statDedupDay = 0
	statDedupLock.Unlock()
}

// 同一用户当天多次调用只计一人，不同用户各计一人，空 dedupKey 被忽略。
func TestRecordDailyActiveDedup(t *testing.T) {
	resetStatFixture(t)
	today := DayNumber(common.GetTimestamp())

	RecordDailyActive(StatKindCall, "u1", 1, "")
	RecordDailyActive(StatKindCall, "u1", 1, "")
	RecordDailyActive(StatKindCall, "u1", 1, "")
	RecordDailyActive(StatKindCall, "u2", 2, "")
	RecordDailyActive(StatKindCall, "", 3, "") // 空 key 不记录

	count, err := CountDailyActive(today, StatKindCall)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// 不同 kind 之间互不影响
	visits, err := CountDailyActive(today, StatKindVisit)
	require.NoError(t, err)
	assert.Equal(t, int64(0), visits)
}

// regionFn 只在当天该主体首次记录时求值一次。
func TestRecordDailyActiveFuncLazyRegion(t *testing.T) {
	resetStatFixture(t)
	calls := 0
	regionFn := func() string {
		calls++
		return "广东"
	}
	RecordDailyActiveFunc(StatKindVisit, "1.2.3.4", 0, regionFn)
	RecordDailyActiveFunc(StatKindVisit, "1.2.3.4", 0, regionFn)
	assert.Equal(t, 1, calls, "region 应仅在首次写库时求值")
}

// 地区分布按人数降序，趋势按天升序聚合。
func TestStatisticsAggregations(t *testing.T) {
	resetStatFixture(t)
	rows := []StatDailyActive{
		{Day: 20260920, Kind: StatKindVisit, DedupKey: "a", Region: "广东"},
		{Day: 20260920, Kind: StatKindVisit, DedupKey: "b", Region: "广东"},
		{Day: 20260920, Kind: StatKindVisit, DedupKey: "c", Region: "北京"},
		{Day: 20260921, Kind: StatKindVisit, DedupKey: "d", Region: "上海"},
		{Day: 20260920, Kind: StatKindCall, DedupKey: "u9", Region: ""},
	}
	for i := range rows {
		require.NoError(t, DB.Create(&rows[i]).Error)
	}

	regions, err := RegionDistribution(20260920, StatKindVisit)
	require.NoError(t, err)
	require.Len(t, regions, 2)
	assert.Equal(t, "广东", regions[0].Region)
	assert.Equal(t, int64(2), regions[0].Count)
	assert.Equal(t, "北京", regions[1].Region)
	assert.Equal(t, int64(1), regions[1].Count)

	trend, err := DailyActiveTrend(20260920, 20260921, StatKindVisit)
	require.NoError(t, err)
	require.Len(t, trend, 2)
	assert.Equal(t, 20260920, trend[0].Day)
	assert.Equal(t, int64(3), trend[0].Count)
	assert.Equal(t, 20260921, trend[1].Day)
	assert.Equal(t, int64(1), trend[1].Count)
}
