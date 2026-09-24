package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm/clause"
)

// StatDailyActive 运营维度的每日去重记录表。
// 每天同一主体（用户 / 访客 IP）同一类型只保留一条，用于统计日活、访问人数、地区分布等。
type StatDailyActive struct {
	Id        int    `json:"id"`
	Day       int    `json:"day" gorm:"index:idx_stat_day_kind_key,unique,priority:1;index:idx_stat_day_kind,priority:1"`
	Kind      int    `json:"kind" gorm:"index:idx_stat_day_kind_key,unique,priority:2;index:idx_stat_day_kind,priority:2"`
	DedupKey  string `json:"dedup_key" gorm:"index:idx_stat_day_kind_key,unique,priority:3;size:64;default:''"`
	UserId    int    `json:"user_id" gorm:"default:0"`
	Region    string `json:"region" gorm:"index;size:64;default:''"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
}

// don't use iota, keep values stable
const (
	StatKindVisit = 1 // 访问网站（按客户端 IP 去重，含匿名访客）
	StatKindCall  = 2 // 调用对话接口（按用户去重，即日活）
)

// DayNumber 把 unix 秒转换为本地时区的 YYYYMMDD 整数，便于跨库按天分组与排序。
func DayNumber(ts int64) int {
	t := time.Unix(ts, 0)
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

var (
	statDedupCache = make(map[string]struct{})
	statDedupLock  sync.Mutex
	statDedupDay   int
)

func statDedupKey(kind int, dedupKey string) string {
	return fmt.Sprintf("%d\x00%s", kind, dedupKey)
}

func statDedupCached(day, kind int, dedupKey string) bool {
	statDedupLock.Lock()
	defer statDedupLock.Unlock()
	if day != statDedupDay {
		statDedupCache = make(map[string]struct{})
		statDedupDay = day
	}
	_, ok := statDedupCache[statDedupKey(kind, dedupKey)]
	return ok
}

func statDedupMark(day, kind int, dedupKey string) {
	statDedupLock.Lock()
	defer statDedupLock.Unlock()
	if day != statDedupDay {
		statDedupCache = make(map[string]struct{})
		statDedupDay = day
	}
	statDedupCache[statDedupKey(kind, dedupKey)] = struct{}{}
}

// RecordDailyActive 记录「当天某主体首次发生某类活动」。
// 进程内缓存做一级去重避免每请求打库；唯一索引 + OnConflict DoNothing 兜底并发与多实例重复。
func RecordDailyActive(kind int, dedupKey string, userId int, region string) {
	RecordDailyActiveFunc(kind, dedupKey, userId, func() string { return region })
}

// RecordDailyActiveFunc 同 RecordDailyActive，但 region 通过回调惰性求值，
// 仅在确实需要写库（当天首次）时才调用，避免每次请求都解析 IP 地区。
func RecordDailyActiveFunc(kind int, dedupKey string, userId int, regionFn func() string) {
	if dedupKey == "" {
		return
	}
	ts := common.GetTimestamp()
	day := DayNumber(ts)
	if statDedupCached(day, kind, dedupKey) {
		return
	}
	region := ""
	if regionFn != nil {
		region = regionFn()
	}
	record := &StatDailyActive{
		Day:       day,
		Kind:      kind,
		DedupKey:  dedupKey,
		UserId:    userId,
		Region:    region,
		CreatedAt: ts,
	}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error; err != nil {
		common.SysLog("failed to record daily active stat: " + err.Error())
		return
	}
	statDedupMark(day, kind, dedupKey)
}

// RegionCount 单个地区的人数。
type RegionCount struct {
	Region string `json:"region"`
	Count  int64  `json:"count"`
}

// DayCount 单日的计数（day 为 YYYYMMDD）。
type DayCount struct {
	Day   int   `json:"day"`
	Count int64 `json:"count"`
}

// CountDailyActive 返回某天某类活动的去重人数。
func CountDailyActive(day, kind int) (int64, error) {
	var count int64
	err := DB.Model(&StatDailyActive{}).
		Where("day = ? AND kind = ?", day, kind).
		Count(&count).Error
	return count, err
}

// RegionDistribution 返回某天某类活动按地区分组的人数，按人数降序。
func RegionDistribution(day, kind int) ([]RegionCount, error) {
	var rows []RegionCount
	err := DB.Model(&StatDailyActive{}).
		Select("region, count(*) as count").
		Where("day = ? AND kind = ?", day, kind).
		Group("region").
		Order("count desc").
		Find(&rows).Error
	return rows, err
}

// DailyActiveTrend 返回 [startDay, endDay] 内每天某类活动的去重人数，按天升序。
func DailyActiveTrend(startDay, endDay, kind int) ([]DayCount, error) {
	var rows []DayCount
	err := DB.Model(&StatDailyActive{}).
		Select("day, count(*) as count").
		Where("day >= ? AND day <= ? AND kind = ?", startDay, endDay, kind).
		Group("day").
		Order("day asc").
		Find(&rows).Error
	return rows, err
}

// CountUsersRegisteredSince 返回 created_at >= ts 的用户数（今日注册）。
func CountUsersRegisteredSince(ts int64) (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("created_at >= ?", ts).Count(&count).Error
	return count, err
}

// CountUsersLoggedInSince 返回 last_login_at >= ts 的用户数（今日登录）。
func CountUsersLoggedInSince(ts int64) (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("last_login_at >= ?", ts).Count(&count).Error
	return count, err
}
