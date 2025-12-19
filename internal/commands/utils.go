package commands

import (
	"log"
	"strconv"
)

func ParseGuildID(guildID string) int64 {
	id, err := strconv.ParseInt(guildID, 10, 64)
	if err != nil {
		log.Printf("Failed to parse guild ID '%s': %v", guildID, err)
		return 0
	}
	return id
}
