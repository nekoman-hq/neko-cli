package config

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	"errors"
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/internal/log"
)

// GetPAT retrieves the GitHub Personal Access Token from the environment.
// returns error if the token is not set.
func GetPAT() (string, error) {
	log.V(log.Config, fmt.Sprintf("Looking up required env variable: %s",
		log.ColorText(log.ColorGreen, "GITHUB_TOKEN"),
	))
	token, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok || token == "" {
		return "", errors.New(
			"environment Variable Missing: \nA GitHub Personal Access Token (GITHUB_TOKEN) is required.\nSet it with: export GITHUB_TOKEN=your_token_here",
		)
	}
	return token, nil
}
