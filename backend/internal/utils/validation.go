package utils

import (
	"strings"
	"unicode/utf8"

	"github.com/andiq123/FindVibeFiber/internal/core/constants"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if len(username) < constants.MinUsernameLength {
		return domain.ErrInvalidInput
	}
	if len(username) > constants.MaxUsernameLength {
		return domain.ErrInvalidInput
	}
	return nil
}

func ValidateQuery(query string) error {
	query = strings.TrimSpace(query)
	if len(query) < constants.MinQueryLength {
		return domain.ErrInvalidInput
	}
	if utf8.RuneCountInString(query) > constants.MaxQueryLength {
		return domain.ErrInvalidInput
	}
	return nil
}

func ValidatePage(page int) error {
	if page < 1 || page > constants.DefaultMaxPageNumber {
		return domain.ErrInvalidInput
	}
	return nil
}

func ValidateSongID(songID string) error {
	if songID == "" {
		return domain.ErrInvalidInput
	}
	if len(songID) > 255 {
		return domain.ErrInvalidInput
	}
	return nil
}

func ValidateUserID(userID string) error {
	if userID == "" {
		return domain.ErrInvalidInput
	}
	if len(userID) > 255 {
		return domain.ErrInvalidInput
	}
	return nil
}
