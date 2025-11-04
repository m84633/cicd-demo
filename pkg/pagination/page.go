package pagination

import "math"

const (
	DefaultPage     = 1
	DefaultPageSize = 10
	MaxPageSize     = 100
)

// PageRequest holds the parameters for a paginated request.
type PageRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// NewPageRequest creates a new PageRequest with default values, ensuring they are within valid ranges.
func NewPageRequest(page, pageSize int) *PageRequest {
	if page <= 0 {
		page = DefaultPage
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return &PageRequest{
		Page:     page,
		PageSize: pageSize,
	}
}

// GetOffset calculates the offset for the database query.
func (p *PageRequest) GetOffset() int {
	return (p.Page - 1) * p.PageSize
}

// GetLimit returns the page size, which is the limit for the database query.
func (p *PageRequest) GetLimit() int {
	return p.PageSize
}

// PageResult holds the data for a paginated response.
type PageResult struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// NewPageResult creates a new PageResult.
func NewPageResult(data interface{}, total int64, req *PageRequest) *PageResult {
	totalPages := 0
	if total > 0 && req.PageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(req.PageSize)))
	}
	return &PageResult{
		Data:       data,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}
}
