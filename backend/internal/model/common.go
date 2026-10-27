package model

// List is what every Find returns: the page plus the total under the filter.
type List[T any] struct {
	Count int32 `json:"count"`
	Items []T   `json:"items"`
}

// Filter is one condition the client sent. Field reaches SQL as an identifier,
// so the repository matches it against a whitelist before using it.
type Filter struct {
	Field string `json:"field"`
	Type  string `json:"type"` // eq | neq | search | gt | gte | lt | lte
	Value string `json:"value"`
}

type SortBy struct {
	Field string `json:"field"`
	Type  string `json:"type"` // asc | desc
}
