package handlers

import (
	"github.com/andiq123/FindVibeFiber/internal/core/ports"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"github.com/gofiber/fiber/v3"
)

type AuthHandler struct {
	authService ports.IAuthService
}

func NewAuthHandler(authService ports.IAuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (ah *AuthHandler) AuthenticateUser(c fiber.Ctx) error {
	username := c.Params("username")

	if err := utils.ValidateUsername(username); err != nil {
		return HandleError(c, err)
	}

	user, err := ah.authService.AuthenticateUser(c.Context(), username)
	if err != nil {
		return HandleError(c, err)
	}

	return c.JSON(user)
}
