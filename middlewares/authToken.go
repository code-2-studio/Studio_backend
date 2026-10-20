package middlewares

import (
	"errors"
	"strings"

	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
)

// Machine-readable 401 reasons returned in the "reason" field so the frontend can react:
// token_expired -> call /auth/refresh, anything else -> send the user to the login screen.
const (
	ReasonMissingToken  = "missing_token"
	ReasonBadHeader     = "bad_authorization_header"
	ReasonTokenExpired  = "token_expired"
	ReasonTokenRevoked  = "token_revoked"
	ReasonTokenInvalid  = "token_invalid"
	ReasonWrongType     = "wrong_token_type"
	ReasonUserNotFound  = "user_not_found"
	reasonUnauthorized  = "unauthorized"
	unauthorizedMessage = "Unauthorized: Authentication required"
)

type authFailure struct {
	reason  string
	message string
}

// unauthorized writes a 401 with a stable reason code.
func unauthorized(c *fiber.Ctx, reason, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"status":  false,
		"code":    fiber.StatusUnauthorized,
		"reason":  reason,
		"message": message,
	})
}

// tokenFromHeader reads the Authorization header. "Bearer <token>" is the standard form; a bare
// token is accepted too because many frontends send it without the scheme.
func tokenFromHeader(c *fiber.Ctx) (token string, present bool) {
	header := strings.TrimSpace(c.Get("Authorization"))
	if header == "" {
		return "", false
	}
	if parts := strings.SplitN(header, " ", 2); len(parts) == 2 {
		if strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1]), true
		}
		return "", true // some other scheme (Basic, ...)
	}
	return header, true
}

// claimsFromRequest validates the access token on the request and says exactly why it failed.
func claimsFromRequest(c *fiber.Ctx) (*utils.TokenClaims, *authFailure) {
	token, present := tokenFromHeader(c)
	if !present {
		return nil, &authFailure{ReasonMissingToken, "Missing Authorization header. Send 'Authorization: Bearer <access_token>'"}
	}
	if token == "" {
		return nil, &authFailure{ReasonBadHeader, "Invalid Authorization header format. Expected 'Bearer <access_token>'"}
	}

	claims, err := utils.ValidateToken(token, utils.TokenTypeAccess)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrExpiredToken):
			return nil, &authFailure{ReasonTokenExpired, "Access token has expired, refresh it or log in again"}
		case errors.Is(err, utils.ErrRevokedToken):
			return nil, &authFailure{ReasonTokenRevoked, "Token has been revoked, log in again"}
		case errors.Is(err, utils.ErrWrongTokenType):
			return nil, &authFailure{ReasonWrongType, "Send the access token, not the refresh token"}
		default:
			return nil, &authFailure{ReasonTokenInvalid, "Access token is invalid"}
		}
	}
	return claims, nil
}
