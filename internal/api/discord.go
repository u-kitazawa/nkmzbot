package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type DiscordUser struct {
	ID         string  `json:"id"`
	Username   string  `json:"username"`
	GlobalName *string `json:"global_name"`
	Avatar     *string `json:"avatar"`
}

type DiscordGuild struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Owner *bool  `json:"owner,omitempty"`
}

func (a *API) getDiscordUser(accessToken string) (*DiscordUser, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "nkmzbot/1.0 (+https://github.com/susu3304/nkmzbot)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord API returned status %d", resp.StatusCode)
	}

	var user DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (a *API) getDiscordGuilds(accessToken string) ([]DiscordGuild, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "nkmzbot/1.0 (+https://github.com/susu3304/nkmzbot)")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord API returned status %d", resp.StatusCode)
	}

	var guilds []DiscordGuild
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return nil, err
	}

	return guilds, nil
}

func getUsername(user *DiscordUser) string {
	if user.GlobalName != nil && *user.GlobalName != "" {
		return *user.GlobalName
	}
	return user.Username
}
