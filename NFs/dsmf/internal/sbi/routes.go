package sbi

import "github.com/gin-gonic/gin"

type Route struct {
	Method  string
	Pattern string
	Handler gin.HandlerFunc
}

func applyRoutes(group *gin.RouterGroup, routes []Route) {
	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.Handler)
		case "POST":
			group.POST(route.Pattern, route.Handler)
		}
	}
}
