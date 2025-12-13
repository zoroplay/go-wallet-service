package common

import "net/http"

type SuccessResponse struct {
	Status  int         `json:"status"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type ErrorResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

func NewSuccessResponse(data interface{}, message string) SuccessResponse {
	return SuccessResponse{
		Status:  http.StatusOK, // Default to 200, similar to NestJS HttpStatus.OK
		Success: true,
		Message: message,
		Data:    data,
	}
}

func NewErrorResponse(message string, data interface{}, status int) ErrorResponse {
	return ErrorResponse{
		Status:  status,
		Success: false,
		Message: message,
		Data:    data,
	}
}
