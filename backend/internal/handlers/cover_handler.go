package handlers

import (
	"net/http"
	"strings"

	"github.com/andiq123/FindVibeFiber/internal/core/services"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"github.com/gofiber/fiber/v3"
)

type CoverHandler struct {
	covers *services.CoverService
}

func NewCoverHandler(covers *services.CoverService) *CoverHandler {
	return &CoverHandler{covers: covers}
}

// GET /cover?q=artist+title → {"image":"https://..."} or {"image":""}.
func (ch *CoverHandler) GetCover(c fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "query parameter 'q' is required"})
	}
	if err := utils.ValidateQuery(q); err != nil {
		return HandleError(c, err)
	}

	img := ch.covers.Lookup(c.Context(), q)
	if img != "" {
		c.Set("Cache-Control", "public, max-age=86400")
	} else {
		c.Set("Cache-Control", "public, max-age=900")
	}
	// ponytail: optional fill — empty art beats 500 when Apple flakes
	return c.JSON(fiber.Map{"image": img})
}
