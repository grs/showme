package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type ActClaim struct {
	Sub string `json:"sub"`
}

type JWTClaims struct {
	Sub   string      `json:"sub"`
	Aud   interface{} `json:"aud"` // Can be string or []string
	Scope string      `json:"scope"`
	Act   *ActClaim   `json:"act"`
}

type JWTInfo struct {
	Subject  string
	Audience string
	Scopes   string
	Actor    string
}

var serviceName string

func formatResponse(service, path string, jwtInfo JWTInfo) string {
	parts := []string{fmt.Sprintf("%s called with path %s", service, path)}

	if jwtInfo.Subject != "" {
		parts = append(parts, fmt.Sprintf("subject %s", jwtInfo.Subject))
	}
	if jwtInfo.Audience != "" {
		parts = append(parts, fmt.Sprintf("audience %s", jwtInfo.Audience))
	}
	if jwtInfo.Scopes != "" {
		parts = append(parts, fmt.Sprintf("scopes %s", jwtInfo.Scopes))
	}
	if jwtInfo.Actor != "" {
		parts = append(parts, fmt.Sprintf("actor %s", jwtInfo.Actor))
	}

	return strings.Join(parts, ", ") + "\n"
}

func extractJWTInfo(authHeader string) JWTInfo {
	emptyInfo := JWTInfo{Subject: "", Audience: "", Scopes: "", Actor: ""}

	if authHeader == "" {
		log.Printf("[JWT] No Authorization header present")
		return emptyInfo
	}

	log.Printf("[JWT] Processing Authorization header (length: %d)", len(authHeader))

	// Remove "Bearer " prefix
	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)

	if token == "" {
		log.Printf("[JWT] Authorization header present but token is empty after removing 'Bearer ' prefix")
		return emptyInfo
	}

	// Show first and last few characters of token for debugging
	tokenPreview := token
	if len(token) > 20 {
		tokenPreview = token[:10] + "..." + token[len(token)-10:]
	}
	log.Printf("[JWT] Token preview: %s (total length: %d)", tokenPreview, len(token))

	// JWT has three parts separated by dots: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		log.Printf("[JWT] ERROR: Invalid JWT structure - expected 3 parts, got %d parts", len(parts))
		return JWTInfo{Subject: "<invalid-jwt>", Audience: "<invalid-jwt>", Scopes: "<invalid-jwt>", Actor: "<invalid-jwt>"}
	}

	log.Printf("[JWT] JWT structure valid: %d parts (header: %d bytes, payload: %d bytes, signature: %d bytes)",
		len(parts), len(parts[0]), len(parts[1]), len(parts[2]))

	// Decode the payload (second part)
	payload := parts[1]
	originalPayloadLen := len(payload)

	// Add padding if necessary
	switch len(payload) % 4 {
	case 2:
		payload += "=="
		log.Printf("[JWT] Added == padding to payload (was %d bytes, now %d bytes)", originalPayloadLen, len(payload))
	case 3:
		payload += "="
		log.Printf("[JWT] Added = padding to payload (was %d bytes, now %d bytes)", originalPayloadLen, len(payload))
	case 0:
		log.Printf("[JWT] No padding needed for payload (%d bytes)", len(payload))
	case 1:
		log.Printf("[JWT] ERROR: Invalid base64 length after padding calculation: %d", len(payload))
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		log.Printf("[JWT] ERROR: Base64 decode failed: %v (payload length: %d)", err, len(payload))
		// Try with standard encoding as fallback
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			log.Printf("[JWT] ERROR: Base64 decode with StdEncoding also failed: %v", err)
			return JWTInfo{Subject: "<decode-error>", Audience: "<decode-error>", Scopes: "<decode-error>", Actor: "<decode-error>"}
		}
		log.Printf("[JWT] WARNING: StdEncoding decode succeeded (RawURLEncoding failed)")
	} else {
		log.Printf("[JWT] Base64 decode successful (%d bytes decoded)", len(decoded))
	}

	log.Printf("[JWT] Decoded payload: %s", string(decoded))

	var claims JWTClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		log.Printf("[JWT] ERROR: JSON unmarshal failed: %v", err)
		return JWTInfo{Subject: "<parse-error>", Audience: "<parse-error>", Scopes: "<parse-error>", Actor: "<parse-error>"}
	}

	// Extract subject
	subject := claims.Sub
	if subject == "" {
		log.Printf("[JWT] WARNING: JWT parsed successfully but 'sub' claim is empty")
	}

	// Extract audience (can be string or []string)
	audience := ""
	if claims.Aud != nil {
		switch aud := claims.Aud.(type) {
		case string:
			audience = aud
		case []interface{}:
			audStrings := make([]string, 0, len(aud))
			for _, a := range aud {
				if audStr, ok := a.(string); ok {
					audStrings = append(audStrings, audStr)
				}
			}
			if len(audStrings) > 0 {
				audience = strings.Join(audStrings, ",")
			}
		}
	}

	// Extract scopes
	scopes := claims.Scope

	// Extract actor from act claim
	actor := ""
	if claims.Act != nil && claims.Act.Sub != "" {
		actor = claims.Act.Sub
	}

	log.Printf("[JWT] Successfully extracted - subject: %s, audience: %s, scopes: %s, actor: %s", subject, audience, scopes, actor)
	return JWTInfo{Subject: subject, Audience: audience, Scopes: scopes, Actor: actor}
}

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	authHeader := r.Header.Get("Authorization")

	log.Printf("[REQUEST] Received request: path=%s, has_auth=%v, remote=%s",
		path, authHeader != "", r.RemoteAddr)

	jwtInfo := extractJWTInfo(authHeader)

	// Parse the path to extract elements
	pathElements := []string{}
	if path != "" && path != "/" {
		elements := strings.Split(strings.Trim(path, "/"), "/")
		for _, elem := range elements {
			if elem != "" {
				pathElements = append(pathElements, elem)
			}
		}
	}

	var response string

	if len(pathElements) == 0 {
		// Terminal case: no more hops
		log.Printf("[CHAIN] Terminal hop reached - returning response")
		response = formatResponse(serviceName, path, jwtInfo)
	} else {
		// Extract the first element as the next hostname
		nextHost := pathElements[0]

		// Create the new path without the first element
		newPathElements := pathElements[1:]
		var newPath string
		if len(newPathElements) == 0 {
			newPath = "/"
		} else {
			newPath = "/" + strings.Join(newPathElements, "/")
		}

		// Make request to next hop
		nextURL := fmt.Sprintf("http://%s%s", nextHost, newPath)

		log.Printf("[CHAIN] Calling next hop: url=%s, forwarding_auth=%v", nextURL, authHeader != "")

		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			log.Printf("[CHAIN] ERROR: Failed to create request to %s: %v", nextHost, err)
			response = fmt.Sprintf("Error creating request to %s: %v\n", nextHost, err)
		} else {
			// Forward the authorization header
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
				log.Printf("[CHAIN] Forwarding Authorization header to %s", nextHost)
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[CHAIN] ERROR: Failed to call %s: %v", nextHost, err)
				response = fmt.Sprintf("Error calling %s: %v\n", nextHost, err)
			} else {
				defer resp.Body.Close()
				log.Printf("[CHAIN] Received response from %s: status=%d", nextHost, resp.StatusCode)
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Printf("[CHAIN] ERROR: Failed to read response from %s: %v", nextHost, err)
					response = fmt.Sprintf("Error reading response from %s: %v\n", nextHost, err)
				} else {
					log.Printf("[CHAIN] Successfully read response from %s (%d bytes)", nextHost, len(body))
					response = string(body)
				}
			}
		}

		// Prepend this hop's information
		response = formatResponse(serviceName, path, jwtInfo) + response
		log.Printf("[CHAIN] Prepended this hop's info and returning response")
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, response)
	log.Printf("[RESPONSE] Sent response (%d bytes) to client", len(response))
}

func main() {
	var nameFlag string
	flag.StringVar(&nameFlag, "name", "", "Service name to use in messages (overrides NAME env var and hostname)")
	flag.Parse()

	// Determine service name: flag > env var > hostname
	if nameFlag != "" {
		serviceName = nameFlag
	} else if envName := os.Getenv("NAME"); envName != "" {
		serviceName = envName
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			serviceName = "unknown"
		} else {
			serviceName = hostname
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Bind address - defaults to all interfaces if not set
	bindAddr := os.Getenv("BIND_ADDR")
	if bindAddr == "" {
		bindAddr = ""  // Empty means all interfaces (0.0.0.0)
	}

	listenAddr := bindAddr + ":" + port

	http.HandleFunc("/", handler)

	if bindAddr == "" {
		log.Printf("Server '%s' starting on all interfaces, port %s", serviceName, port)
	} else {
		log.Printf("Server '%s' starting on %s", serviceName, listenAddr)
	}

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
