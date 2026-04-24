package sbi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDataServiceRoutes() []Route {
	return []Route{
		{
			Method:  http.MethodPost,
			Pattern: "/tasks",
			APIFunc: s.apiPostDataServiceTask,
		},
	}
}

func (s *Server) apiPostDataServiceTask(gc *gin.Context) {
	body, err := gc.GetRawData()
	if err != nil {
		gc.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := s.Processor().ForwardDataServiceTask(body, gc.GetHeader("Content-Type"))
	for key, values := range resp.Headers {
		for _, value := range values {
			gc.Header(key, value)
		}
	}
	if resp.Body == nil {
		gc.Status(resp.Status)
		return
	}
	body, ok := resp.Body.([]byte)
	if !ok {
		gc.JSON(http.StatusInternalServerError, gin.H{"error": "invalid DSMF response body"})
		return
	}
	gc.Data(resp.Status, "application/json", body)
}
