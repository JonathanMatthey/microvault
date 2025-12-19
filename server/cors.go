package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// buildCORSConfig constructs the Echo CORS config using values from config,
// with sensible defaults for dev/prod if none are provided.
func buildCORSConfig(cfg *Config) middleware.CORSConfig {
	origins := cfg.CORS.AllowOrigins
	if len(origins) == 0 {
		origins = []string{
			"http://localhost:5173", "http://127.0.0.1:5173",
			"http://localhost:5174", "http://127.0.0.1:5174",
			"https://vault.skatkis-tech.net", "https://content.skatkis-tech.net",
		}
	}

	return middleware.CORSConfig{
		AllowOrigins:     origins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-User-ID", "X-Filename"},
		ExposeHeaders:    []string{echo.HeaderContentType},
		AllowCredentials: true,
		MaxAge:           3600,
	}
}
