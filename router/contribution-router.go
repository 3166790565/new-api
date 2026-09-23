package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// registerContributionRoutes wires the channel-contribution feature. The self
// group is for any authenticated user (submit/list/edit/withdraw a submission,
// view earnings, transfer the earned share into spendable quota); the admin
// group is for reviewers (queue/detail/approve/reject, contributor overview,
// stats, per-user share override).
func registerContributionRoutes(apiRouter *gin.RouterGroup) {
	selfRoute := apiRouter.Group("/contribution")
	selfRoute.Use(middleware.UserAuth())
	{
		selfRoute.POST("/", controller.SubmitContribution)
		selfRoute.POST("/fetch-models", controller.FetchContributionModels)
		selfRoute.POST("/test-model", controller.TestContributionModel)
		selfRoute.GET("/self", controller.GetSelfContributions)
		selfRoute.GET("/share", controller.GetSelfContributionShare)
		selfRoute.GET("/earnings", controller.GetSelfContributionEarnings)
		selfRoute.POST("/transfer", controller.TransferContributionEarnings)
		selfRoute.GET("/detail/:id", controller.GetSelfContribution)
		selfRoute.PUT("/detail/:id", controller.UpdateSelfContribution)
		selfRoute.DELETE("/detail/:id", controller.WithdrawSelfContribution)
	}

	adminRoute := apiRouter.Group("/contribution/admin")
	adminRoute.Use(middleware.AdminAuth())
	{
		adminRoute.GET("/", controller.GetContributionsForReview)
		adminRoute.GET("/stats", controller.GetContributionStats)
		adminRoute.GET("/contributors", controller.GetContributors)
		adminRoute.GET("/:id", controller.GetContributionDetail)
		adminRoute.POST("/review", controller.ReviewContribution)
		adminRoute.PUT("/share", controller.SetContributorSharePercent)
	}
}
