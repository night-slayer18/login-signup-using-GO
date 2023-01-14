package routes

import (
	controller "github.com/night-slayer18/golang-jwt-project/controllers"
	"github.com/night-slayer18/golang-jwt-project/middleware"

	"github.com/gin-gonic/gin"
)

func UserRoutes(incomingRoutes *gin.Engine) {
	incomingRoutes.Use(middleware.Authenticate())
	incomingRoutes.GET("/users", controller.GetUsers())
	incomingRoutes.DELETE("/users/:user_id", controller.GetUser())
	//incomingRoutes.DELETE("/users/delete", controller.deleteUser())
	incomingRoutes.POST("/upload/:image_name", controller.UploadFile())
	incomingRoutes.GET("/download/:image_name", controller.DownloadFile())
}
