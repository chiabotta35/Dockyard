package registry

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	dockerCliConfig "github.com/docker/cli/cli/config"
	dockerConfigConfigfile "github.com/docker/cli/cli/config/configfile"
	dockerConfigCredentials "github.com/docker/cli/cli/config/credentials"
	dockerConfig "github.com/docker/cli/cli/config/types"

	"github.com/dockyard/dockyard/pkg/registry/auth"
)

// Errors for registry authentication operations.
var (
	// errUnsetRegAuthVars indicates registry auth environment variables (REPO_USER, REPO_PASS) are not set.
	errUnsetRegAuthVars = errors.New(
		"registry auth environment variables (REPO_USER, REPO_PASS) not set",
	)
	// errFailedGetRegistryAddress indicates a failure to extract the registry address from an image reference.
	errFailedGetRegistryAddress = errors.New("failed to get registry address")
	// errFailedLoadDockerConfig indicates a failure to load the Docker configuration file.
	errFailedLoadDockerConfig = errors.New("failed to load Docker config")
	// errFailedMarshalAuthConfig indicates a failure to marshal the auth config to JSON.
	errFailedMarshalAuthConfig = errors.New("failed to marshal auth config to JSON")
)

// EncodedAuth attempts to retrieve encoded authentication credentials for a given image name.
//
// It checks environment variables first, then falls back to the Docker config file if needed.
//
// Parameters:
//   - ref: Image reference string (e.g., "docker.io/library/alpine").
//
// Returns:
//   - string: Base64-encoded credentials string if successful, empty if none found.
//   - error: Non-nil if both methods fail, nil on success or if no credentials are available.
func EncodedAuth(imageName string) (string, error) {
	// Set up logging fields for tracking.
	fields := logrus.Fields{
		"image_ref": imageName,
	}

	logrus.WithFields(fields).Debug("Attempting to retrieve auth credentials")

	// Try environment variables first.
	credentials, err := EncodedEnvAuth()
	if err != nil {
		// Fallback to config file if env vars are unavailable.
		logrus.WithError(err).
			WithFields(fields).
			Debug("Environment auth not available, trying config file")

		credentials, err = EncodedConfigCredentials(imageName)
	}

	if err == nil {
		logrus.WithFields(fields).Debug("Successfully retrieved encoded auth credentials")
	}

	return credentials, err
}

// EncodedEnvAuth checks for REPO_USER and REPO_PASS environment variables and encodes them.
//
// It returns an error if these variables are not set.
//
// Returns:
//   - string: Base64-encoded auth string if credentials are found.
//   - error: Non-nil if env vars are missing, nil on success.
func EncodedEnvAuth() (string, error) {
	// Retrieve username and password from environment.
	username := os.Getenv("REPO_USER")
	password := os.Getenv("REPO_PASS")

	// Check if both variables are set.
	if username != "" && password != "" {
		credentials := dockerConfig.AuthConfig{
			Username: username,
			Password: password,
		}

		logrus.WithFields(logrus.Fields{
			"username": username,
		}).Debug("Loaded auth credentials from environment")

		// Log sensitive password only in trace mode.
		if logrus.GetLevel() == logrus.TraceLevel {
			logrus.WithFields(logrus.Fields{
				"username": username,
				"password": password,
			}).Trace("Using environment credentials")
		}

		// Encode and return the auth config.
		return EncodeCredentials(credentials)
	}

	// Return error if variables are missing.
	logrus.Debug("Environment auth variables not set")

	return "", errUnsetRegAuthVars
}

// dockerConfigJSON is a minimal struct for directly reading Docker config.json
// without depending on the Docker CLI library. This is the ultimate fallback
// when the CLI library fails to resolve credentials (e.g. missing credsStore binary).
type dockerConfigJSON struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

// EncodedConfigCredentials retrieves authentication credentials from the Docker config file.
//
// The Docker config must be mounted on the container.
// Uses a three-layer fallback: Docker CLI credential store → Docker CLI auths map → direct JSON read.
//
// Parameters:
//   - imageRef: Image reference string for registry lookup.
//
// Returns:
//   - string: Base64-encoded credentials string if found, empty if none.
//   - error: Non-nil if config loading or address retrieval fails, nil on success or if no auth is found.
func EncodedConfigCredentials(imageRef string) (string, error) {
	fields := logrus.Fields{
		"image_ref": imageRef,
	}

	server, err := auth.GetRegistryAddress(imageRef)
	if err != nil {
		logrus.WithError(err).WithFields(fields).Debug("Failed to get registry address")
		return "", fmt.Errorf("%w: %w", errFailedGetRegistryAddress, err)
	}

	configDir := os.Getenv("DOCKER_CONFIG")
	if configDir == "" {
		configDir = "/"
		logrus.WithFields(fields).Debug("No DOCKER_CONFIG set, using default directory")
	}

	configPath := filepath.Join(configDir, "config.json")

	// Layer 1: Try Docker CLI credential store / file store.
	configFile, cliErr := dockerCliConfig.Load(configDir)
	if cliErr == nil {
		credStore := CredentialsStore(*configFile)
		credentials, _ := credStore.Get(server)

		if credentials != (dockerConfig.AuthConfig{}) {
			logrus.WithFields(fields).WithFields(logrus.Fields{
				"server":      server,
				"config_file": configFile.Filename,
				"username":    credentials.Username,
			}).Debug("Loaded auth from Docker CLI credential store")
			return EncodeCredentials(credentials)
		}

		// Layer 2: Docker CLI auths map direct read (handles missing credsStore binary).
		if auths, ok := configFile.AuthConfigs[server]; ok && auths.Auth != "" {
			decoded, decodeErr := base64.StdEncoding.DecodeString(auths.Auth)
			if decodeErr == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) == 2 {
					logrus.WithFields(fields).WithFields(logrus.Fields{
						"server": server,
					}).Debug("Layer 2: read auth from Docker CLI auths map")
					return EncodeCredentials(dockerConfig.AuthConfig{
						Username: parts[0],
						Password: parts[1],
					})
				}
			}
		}
	} else {
		logrus.WithError(cliErr).WithFields(fields).Debug("Docker CLI Load failed, trying direct JSON read")
	}

	// Layer 3: Direct JSON read — bypass Docker CLI entirely.
	// This handles edge cases where the CLI library doesn't parse the config correctly.
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		var cfg dockerConfigJSON
		if jsonErr := json.Unmarshal(data, &cfg); jsonErr == nil {
			// Try exact match first.
			if entry, ok := cfg.Auths[server]; ok && entry.Auth != "" {
				if cred, decodeErr := decodeAuthEntry(entry.Auth); decodeErr == nil {
					logrus.WithFields(fields).WithFields(logrus.Fields{
						"server": server,
					}).Debug("Layer 3: read auth from direct JSON parse of config file")
					return EncodeCredentials(cred)
				}
			}
			// Try legacy URL-style keys.
			for key, entry := range cfg.Auths {
				if entry.Auth != "" && server == convertToHostname(key) {
					if cred, decodeErr := decodeAuthEntry(entry.Auth); decodeErr == nil {
						logrus.WithFields(fields).WithFields(logrus.Fields{
							"server": server,
							"key":    key,
						}).Debug("Layer 3: read auth via legacy key match")
						return EncodeCredentials(cred)
					}
				}
			}
		}
	}

	logrus.WithFields(fields).WithFields(logrus.Fields{
		"server":      server,
		"config_path": configPath,
	}).Debug("No credentials found after all fallback layers")

	return "", nil
}

// decodeAuthEntry decodes a base64 "auth" field (username:password) from a Docker config.
func decodeAuthEntry(authStr string) (dockerConfig.AuthConfig, error) {
	decoded, err := base64.StdEncoding.DecodeString(authStr)
	if err != nil {
		return dockerConfig.AuthConfig{}, err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return dockerConfig.AuthConfig{}, fmt.Errorf("invalid auth format")
	}
	return dockerConfig.AuthConfig{
		Username: parts[0],
		Password: parts[1],
	}, nil
}

// convertToHostname strips scheme and path from a URL-like string to extract hostname.
func convertToHostname(maybeURL string) string {
	s := strings.TrimPrefix(maybeURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	host, _, _ := strings.Cut(s, "/")
	return host
}

// CredentialsStore returns a new credentials store based on the configuration file settings.
//
// It selects a native or file-based store depending on the config.
//
// Parameters:
//   - configFile: Docker configuration file.
//
// Returns:
//   - dockerConfigCredentials.Store: Configured credentials store.
func CredentialsStore(configFile dockerConfigConfigfile.ConfigFile) dockerConfigCredentials.Store {
	// Use native store if a credentials store is specified.
	if configFile.CredentialsStore != "" {
		return dockerConfigCredentials.NewNativeStore(&configFile, configFile.CredentialsStore)
	}

	// Default to file-based store otherwise.
	return dockerConfigCredentials.NewFileStore(&configFile)
}

// EncodeCredentials Base64 encodes an AuthConfig struct for HTTP transmission.
//
// It marshals the struct to JSON and applies URL-safe base64 encoding.
//
// Parameters:
//   - authConfig: Authentication configuration to encode.
//
// Returns:
//   - string: Base64-encoded auth string if successful.
//   - error: Non-nil if marshaling fails, nil on success.
func EncodeCredentials(authConfig dockerConfig.AuthConfig) (string, error) {
	// Set up logging fields with username for tracking.
	fields := logrus.Fields{
		"username": authConfig.Username,
	}

	// Marshal the auth config to JSON.
	//nolint:gosec // G117: This is the expected standard Docker auth format
	buf, err := json.Marshal(authConfig)
	if err != nil {
		logrus.WithError(err).WithFields(fields).Debug("Failed to marshal auth config to JSON")

		return "", fmt.Errorf("%w: %w", errFailedMarshalAuthConfig, err)
	}

	// Encode the JSON to base64url (RFC4648 section 5) for safe transmission.
	encoded := base64.URLEncoding.EncodeToString(buf)

	logrus.WithFields(fields).Debug("Encoded auth config")

	return encoded, nil
}
