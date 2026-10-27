// Package dto holds the HTTP-only request and response shapes. A response that
// equals a domain model is returned as that model.
package dto

type Empty struct{}

type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Error struct {
	Error ResponseError `json:"error"`
}

type Health struct {
	Status string `json:"status"`
}
