package routes

import (
	controller "github.com/night-slayer18/DriverAppBackEnd/controllers"

	"github.com/gin-gonic/gin"
)

func AuthRoutes(incomingRoutes *gin.Engine) {
	incomingRoutes.POST("users/signup", controller.DriverSignup())
	incomingRoutes.POST("users/login", controller.DriverLogin())
}
