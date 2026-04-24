package sbi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/dsmf/internal/processor"
)

func (s *Server) routes() []Route {
	return []Route{
		{
			Method:  http.MethodPost,
			Pattern: "/tasks",
			Handler: s.postTask,
		},
		{
			Method:  http.MethodGet,
			Pattern: "/tasks/:taskId",
			Handler: s.getTask,
		},
	}
}

func (s *Server) postTask(c *gin.Context) {
	var req processor.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, status, err := s.processor.CreateTask(req)
	if err != nil {
		c.JSON(status, gin.H{
			"error": err.Error(),
			"task":  task,
		})
		return
	}
	c.JSON(status, task)
}

func (s *Server) getTask(c *gin.Context) {
	task := s.processor.GetTask(c.Param("taskId"))
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}
