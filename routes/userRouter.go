package routes

import (
	controller "github.com/night-slayer18/DriverAppBackEnd/controllers"
	"github.com/night-slayer18/DriverAppBackEnd/middleware"

	"github.com/gin-gonic/gin"
)

func UserRoutes(incomingRoutes *gin.Engine) {
	incomingRoutes.Use(middleware.Authenticate())
	incomingRoutes.GET("/users", controller.GetUsers())
	incomingRoutes.DELETE("/users/:user_id", controller.GetUser())
}
