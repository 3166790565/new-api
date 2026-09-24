package middleware

import (
	"github.com/QuantumNous/new-api/common/ipgeo"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// RecordWebVisit 在返回前端页面（index.html）时记录一次站点访问。
// 按客户端 IP 每日去重（含未登录匿名访客），并在当天该 IP 首次访问时解析其地区。
// 仅应在确认要返回页面时调用，避免把静态资源、/v1、/api 请求计入访问量。
func RecordWebVisit(c *gin.Context) {
	ip := c.ClientIP()
	if ip == "" {
		return
	}
	userId := c.GetInt("id")
	model.RecordDailyActiveFunc(model.StatKindVisit, ip, userId, func() string {
		return ipgeo.ResolveRegion(ip)
	})
}
