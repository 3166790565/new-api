package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	statTrendDefaultDays = 7
	statTrendMaxDays     = 90
)

func dayCountMap(rows []model.DayCount) map[int]int64 {
	result := make(map[int]int64, len(rows))
	for _, r := range rows {
		result[r.Day] = r.Count
	}
	return result
}

// GetStatisticsOverview 返回管理员统计看板数据：今日 KPI、IP 地区分布、近 N 天趋势。
func GetStatisticsOverview(c *gin.Context) {
	days := statTrendDefaultDays
	if v, err := strconv.Atoi(c.Query("days")); err == nil && v > 0 {
		days = min(v, statTrendMaxDays)
	}

	now := time.Unix(common.GetTimestamp(), 0)
	y, m, d := now.Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	todayStartTs := todayStart.Unix()
	tomorrowStartTs := todayStart.AddDate(0, 0, 1).Unix()
	todayNum := model.DayNumber(todayStartTs)

	registrations, err := model.CountUsersRegisteredSince(todayStartTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	logins, err := model.CountUsersLoggedInSince(todayStartTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	visitors, err := model.CountDailyActive(todayNum, model.StatKindVisit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	activeUsers, err := model.CountDailyActive(todayNum, model.StatKindCall)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	callCount, err := model.CountConsumeLogs(todayStartTs, tomorrowStartTs-1)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	regionRows, err := model.RegionDistribution(todayNum, model.StatKindVisit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	startDayNum := model.DayNumber(todayStart.AddDate(0, 0, -(days - 1)).Unix())
	visitTrend, err := model.DailyActiveTrend(startDayNum, todayNum, model.StatKindVisit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	callerTrend, err := model.DailyActiveTrend(startDayNum, todayNum, model.StatKindCall)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	visitMap := dayCountMap(visitTrend)
	callerMap := dayCountMap(callerTrend)

	trend := make([]gin.H, 0, days)
	for i := days - 1; i >= 0; i-- {
		dt := todayStart.AddDate(0, 0, -i)
		dayNum := model.DayNumber(dt.Unix())
		dayCallCount, err := model.CountConsumeLogs(dt.Unix(), dt.AddDate(0, 0, 1).Unix()-1)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		trend = append(trend, gin.H{
			"day":          dt.Format("2006-01-02"),
			"active_users": callerMap[dayNum],
			"visitors":     visitMap[dayNum],
			"call_count":   dayCallCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"today": gin.H{
				"registrations": registrations,
				"logins":        logins,
				"visitors":      visitors,
				"active_users":  activeUsers,
				"call_count":    callCount,
			},
			"region_distribution": regionRows,
			"trend":               trend,
		},
	})
}
