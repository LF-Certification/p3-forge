package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// UsernameClaim is the LFID username claim in education API access tokens.
// The delivery service returns 401 without it.
const UsernameClaim = "http://lfx.dev/claims/username"

// idTokenUsernameClaim is the LFID username claim in Auth0 id_tokens. It
// uses a different namespace than the access token.
const idTokenUsernameClaim = "https://sso.linuxfoundation.org/claims/username" // #nosec G101 -- claim name URL, not a credential

// Claims are the access token fields that `lfed auth status` shows. They
// come from an unverified decode; the API verifies the token on each call.
type Claims struct {
	Username string
	Subject  string
	Issuer   string
	Expiry   time.Time
}

// DecodeClaims decodes the payload of a JWT without signature checks.
func DecodeClaims(token string) (Claims, error) {
	payload, err := jwtPayload(token)
	if err != nil {
		return Claims{}, err
	}
	var raw struct {
		Username string `json:"http://lfx.dev/claims/username"`
		Subject  string `json:"sub"`
		Issuer   string `json:"iss"`
		Exp      int64  `json:"exp"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Claims{}, fmt.Errorf("decode token claims: %w", err)
	}
	claims := Claims{Username: raw.Username, Subject: raw.Subject, Issuer: raw.Issuer}
	if raw.Exp != 0 {
		claims.Expiry = time.Unix(raw.Exp, 0)
	}
	return claims, nil
}

// IdentityFromIDToken returns a display name from an id_token for the
// "Logged in as" message. It returns "" when nothing usable is present.
func IdentityFromIDToken(idToken string) string {
	payload, err := jwtPayload(idToken)
	if err != nil {
		return ""
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	username, _ := claims[idTokenUsernameClaim].(string)
	email, _ := claims["email"].(string)
	sub, _ := claims["sub"].(string)
	switch {
	case username != "" && email != "":
		return fmt.Sprintf("%s (%s)", username, email)
	case username != "":
		return username
	case email != "":
		return email
	default:
		return sub
	}
}

// jwtPayload returns the decoded second segment of a compact JWT.
func jwtPayload(token string) ([]byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode token payload: %w", err)
	}
	return payload, nil
}
