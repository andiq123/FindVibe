package handlers

import (
	"errors"
	"net/http"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/gofiber/fiber/v3"
)

// HandleError maps domain errors to a stable JSON envelope for iOS/web.
// Wrapped fmt.Errorf("…: %w", err) messages are unwrapped to the domain text.
func HandleError(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	status := http.StatusInternalServerError
	msg := "internal error"

	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		msg = domain.ErrNotFound.Error()
	case errors.Is(err, domain.ErrAlreadyExists):
		status = http.StatusConflict
		msg = domain.ErrAlreadyExists.Error()
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = domain.ErrInvalidInput.Error()
	case errors.Is(err, domain.ErrUnavailable):
		status = http.StatusServiceUnavailable
		msg = domain.ErrUnavailable.Error()
	}

	return c.Status(status).JSON(fiber.Map{"error": msg})
}
