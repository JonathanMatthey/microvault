package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type GoogleClaims struct {
	Email string `json:"email"`
	Sub   string `json:"sub"`
	Aud   string `json:"aud"`
	Iss   string `json:"iss"`
	jwt.RegisteredClaims
}

type UserContext struct {
	Email  string
	UserID string
}

var (
	googleCerts    map[string]*rsa.PublicKey
	certsMutex     sync.RWMutex
	certsLastFetch time.Time
)

// InitGoogleAuth initializes Google OAuth verification
func InitGoogleAuth() error {
	if err := fetchGoogleCerts(); err != nil {
		return err
	}

	// Refresh certificates every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := fetchGoogleCerts(); err != nil {
				log.Printf("Failed to refresh Google certs: %v", err)
			}
		}
	}()

	return nil
}

// fetchGoogleCerts fetches Google's public keys
func fetchGoogleCerts() error {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v1/certs")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var certs map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&certs); err != nil {
		return err
	}

	// Parse certificates (they are in PEM format)
	newCerts := make(map[string]*rsa.PublicKey)
	for kid, certPEM := range certs {
		pubKey, err := parseCertificate(certPEM)
		if err != nil {
			log.Printf("Failed to parse certificate %s: %v", kid, err)
			continue
		}
		newCerts[kid] = pubKey
	}

	if len(newCerts) == 0 {
		return fmt.Errorf("no valid certificates parsed")
	}

	certsMutex.Lock()
	googleCerts = newCerts
	certsLastFetch = time.Now()
	certsMutex.Unlock()

	log.Printf("Fetched %d Google certificates", len(newCerts))
	return nil
}

// parseCertificate parses a PEM certificate string into an RSA public key
func parseCertificate(certPEM string) (*rsa.PublicKey, error) {
	// Google returns certificates as X.509 certificates in PEM format
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate does not contain RSA public key")
	}

	return rsaKey, nil
}

// VerifyGoogleToken verifies a Google ID token and returns claims
func VerifyGoogleToken(tokenString string) (*UserContext, error) {
	cfg := GetConfig()
	if !cfg.Auth.Enabled {
		return nil, fmt.Errorf("authentication is disabled")
	}

	token, err := jwt.ParseWithClaims(tokenString, &GoogleClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is RS256
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing key id in token header")
		}

		certsMutex.RLock()
		pubKey, ok := googleCerts[kid]
		certsMutex.RUnlock()

		if !ok {
			// Try to refresh certificates if key not found
			if err := fetchGoogleCerts(); err == nil {
				certsMutex.RLock()
				pubKey, ok = googleCerts[kid]
				certsMutex.RUnlock()
			}
			if !ok {
				return nil, fmt.Errorf("unknown key id: %s", kid)
			}
		}

		return pubKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*GoogleClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	// Validate token claims
	if claims.Aud != cfg.Auth.GoogleClientID {
		return nil, fmt.Errorf("invalid audience: expected %s, got %s", cfg.Auth.GoogleClientID, claims.Aud)
	}

	if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Iss)
	}

	if claims.Email == "" {
		return nil, fmt.Errorf("email not in token")
	}

	return &UserContext{
		Email:  claims.Email,
		UserID: claims.Sub,
	}, nil
}

// AuthMiddleware verifies Google OAuth tokens
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		user, err := VerifyGoogleToken(tokenString)
		if err != nil {
			log.Printf("Token verification failed: %v", err)
			http.Error(w, fmt.Sprintf("invalid token: %v", err), http.StatusUnauthorized)
			return
		}

		// Add user context to request
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext extracts user info from request context
func GetUserFromContext(r *http.Request) *UserContext {
	user := r.Context().Value("user")
	if user != nil {
		return user.(*UserContext)
	}
	return nil
}
