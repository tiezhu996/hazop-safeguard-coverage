package router

import (
	"hazop-safeguard-coverage/backend/internal/constants"
	"hazop-safeguard-coverage/backend/internal/handler"
	"hazop-safeguard-coverage/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterSafeguardRoutes(api *gin.RouterGroup, h *handler.SafeguardHandler) {
	group := api.Group("/safeguards")
	group.GET("", middleware.RequirePermission(constants.PermissionRead), h.List)
	group.GET("/:id", middleware.RequirePermission(constants.PermissionRead), h.Get)
	write := middleware.RequireRoles(constants.RoleAdmin, constants.RoleProcessEngineer)
	group.POST("", write, h.Create)
	group.PUT("/:id", write, h.Update)
	confirm := middleware.RequireRoles(constants.RoleAdmin, constants.RoleProcessEngineer)
	group.POST("/:id/verify", confirm, h.Verify)
	group.POST("/:id/invalidate", confirm, h.Invalidate)
	group.POST("/:id/restore", confirm, h.Restore)
}
