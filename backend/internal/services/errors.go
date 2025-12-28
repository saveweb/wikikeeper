package services

import (
	"errors"
	"fmt"
)

// Service-level error definitions
var (
	ErrMediaWikiNotFound    = errors.New("mediawiki API not found")
	ErrMediaWikiUnavailable = errors.New("mediawiki API unavailable")
	ErrInvalidResponse      = errors.New("invalid API response")
	ErrInvalidWikiURL       = errors.New("invalid wiki URL")
	ErrWikiNotFound         = errors.New("wiki not found")
	ErrWikiDeleted          = errors.New("wiki deleted as duplicate")
)

// ServiceError wraps errors with context about the operation
type ServiceError struct {
	Op  string // Operation that failed
	URL string // URL being processed
	Err error  // Underlying error
}

// Error returns a formatted error message
func (e *ServiceError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("[%s] %s: %v", e.URL, e.Op, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewMediaWikiError creates a new MediaWiki-related error
func NewMediaWikiError(op, url string, err error) *ServiceError {
	return &ServiceError{
		Op:  op,
		URL: url,
		Err: err,
	}
}

// NewCollectorError creates a new collector-related error
func NewCollectorError(op string, err error) *ServiceError {
	return &ServiceError{
		Op:  op,
		Err: err,
	}
}
