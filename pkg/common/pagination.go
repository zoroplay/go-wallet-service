package common

import "math"

type PaginationResult struct {
	Message     string      `json:"message"`
	Data        interface{} `json:"data"`
	Count       int64       `json:"count"`
	CurrentPage int         `json:"currentPage"`
	NextPage    int         `json:"nextPage"`
	PrevPage    int         `json:"prevPage"`
	LastPage    int         `json:"lastPage"`
}

// PaginateResponse creates a paginated response.
// data: The slice of results
// total: The total count of items
// page: The current page number
// limit: The number of items per page
// message: Optional message, defaults to "success"
func PaginateResponse(data interface{}, total int64, page int, limit int, message string) PaginationResult {
	if message == "" {
		message = "success"
	}

	lastPage := 0
	if limit > 0 {
		lastPage = int(math.Ceil(float64(total) / float64(limit)))
	}

	nextPage := page + 1
	if nextPage > lastPage {
		nextPage = 0
	}

	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 0
	}

	return PaginationResult{
		Message:     message,
		Data:        data,
		Count:       total,
		CurrentPage: page,
		NextPage:    nextPage,
		PrevPage:    prevPage,
		LastPage:    lastPage,
	}
}
