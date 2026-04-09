package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the standard API response structure
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta contains pagination metadata
type Meta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// Success sends a success response
func Success(c *gin.Context, code int, data interface{}) {
	c.JSON(code, Response{
		Success: true,
		Data:    data,
	})
}

// Error sends an error response
func Error(c *gin.Context, code int, message string) {
	c.JSON(code, Response{
		Success: false,
		Error:   message,
	})
}

// WithMeta sends a success response with pagination metadata
func WithMeta(c *gin.Context, code int, data interface{}, meta *Meta) {
	c.JSON(code, Response{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// OK sends 200 OK response
func OK(c *gin.Context, data interface{}) {
	Success(c, http.StatusOK, data)
}

// Created sends 201 Created response
func Created(c *gin.Context, data interface{}) {
	Success(c, http.StatusCreated, data)
}

// BadRequest sends 400 Bad Request response
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized sends 401 Unauthorized response
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden sends 403 Forbidden response
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound sends 404 Not Found response
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// Conflict sends 409 Conflict response
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, message)
}

// InternalServerError sends 500 Internal Server Error response
func InternalServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}
